package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"log"
	"net/http"
	"time"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type openaiModel struct {
	client *openai.Client
	name   string
}

// NewOpenAIModel creates a new model.LLM backed by the sashabaranov/go-openai client.
// If baseURL is provided, it overrides the default OpenAI API endpoint.
func NewOpenAIModel(apiKey string, baseURL string, modelName string) (model.LLM, error) {
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	// DeepSeek APIs can be slow, especially during high load. Default HTTP client might timeout too early.
	config.HTTPClient = &http.Client{
		Timeout: 10 * time.Minute,
	}

	client := openai.NewClientWithConfig(config)
	return &openaiModel{
		client: client,
		name:   modelName,
	}, nil
}

func (m *openaiModel) Name() string {
	return m.name
}

func (m *openaiModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	if stream {
		return m.generateStream(ctx, req)
	}

	return func(yield func(*model.LLMResponse, error) bool) {
		resp, err := m.generate(ctx, req)
		yield(resp, err)
	}
}

func (m *openaiModel) generate(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	chatReq := m.toOpenAIRequest(req)
	resp, err := m.client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call openai model: %w", err)
	}
	return m.toLLMResponse(&resp), nil
}

func (m *openaiModel) generateStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		chatReq := m.toOpenAIStreamRequest(req)

		stream, err := m.client.CreateChatCompletionStream(ctx, chatReq)
		if err != nil {
			yield(nil, fmt.Errorf("failed to start stream openai: %w", err))
			return
		}
		defer stream.Close()

		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				yield(nil, err)
				return
			}

			llmResp := m.toLLMResponseChunk(&chunk)
			if !yield(llmResp, nil) {
				return // Consumer stopped
			}
		}
	}
}

func (m *openaiModel) toOpenAIRequest(req *model.LLMRequest) openai.ChatCompletionRequest {
	chatReq := openai.ChatCompletionRequest{
		Model: m.name,
	}

	chatReq.Messages = m.convertMessages(req)

	if req.Config != nil {
		if req.Config.Temperature != nil {
			chatReq.Temperature = *req.Config.Temperature
		}
		if req.Config.TopP != nil {
			chatReq.TopP = *req.Config.TopP
		}
		if req.Config.MaxOutputTokens > 0 {
			// Clamp max_tokens to valid range [1, 8192] for DeepSeek API compatibility
			maxTokens := int(req.Config.MaxOutputTokens)
			if maxTokens > 8192 {
				maxTokens = 8192
			}
			if maxTokens < 1 {
				maxTokens = 1
			}
			chatReq.MaxTokens = maxTokens
		}
		if len(req.Config.StopSequences) > 0 {
			chatReq.Stop = req.Config.StopSequences
		}
		if req.Config.PresencePenalty != nil {
			chatReq.PresencePenalty = *req.Config.PresencePenalty
		}
		if req.Config.FrequencyPenalty != nil {
			chatReq.FrequencyPenalty = *req.Config.FrequencyPenalty
		}
		if req.Config.ResponseMIMEType == "application/json" {
			chatReq.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}
		}

		chatReq.Tools = m.convertTools(req.Config.Tools)
	}

	return chatReq
}

func (m *openaiModel) toOpenAIStreamRequest(req *model.LLMRequest) openai.ChatCompletionRequest {
	chatReq := m.toOpenAIRequest(req)
	chatReq.Stream = true
	return chatReq
}

func (m *openaiModel) convertTools(tools []*genai.Tool) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}
	var res []openai.Tool
	for _, t := range tools {
		if t.FunctionDeclarations != nil {
			for _, fd := range t.FunctionDeclarations {
				b, _ := json.Marshal(fd.Parameters)
				var params map[string]interface{}
				json.Unmarshal(b, &params)

				res = append(res, openai.Tool{
					Type: openai.ToolTypeFunction,
					Function: &openai.FunctionDefinition{
						Name:        fd.Name,
						Description: fd.Description,
						Parameters:  params, // passing raw map is fine for go-openai usually, but just in case:
					},
				})
			}
		}
	}
	return res
}

func (m *openaiModel) convertMessages(req *model.LLMRequest) []openai.ChatCompletionMessage {
	var messages []openai.ChatCompletionMessage

	// Add system instruction if present
	if req.Config != nil && req.Config.SystemInstruction != nil && len(req.Config.SystemInstruction.Parts) > 0 {
		text := ""
		for _, part := range req.Config.SystemInstruction.Parts {
			if part.Text != "" {
				text += part.Text
			}
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: text,
		})
	}

	// Add contents
	for _, content := range req.Contents {
		role := openai.ChatMessageRoleUser
		if content.Role == "model" {
			role = openai.ChatMessageRoleAssistant
		}

		var asstMsg openai.ChatCompletionMessage
		asstMsg.Role = role
		asstHasContent := false

		var toolResponses []openai.ChatCompletionMessage

		text := ""
		for _, part := range content.Parts {
			if part.Text != "" {
				text += part.Text
				asstHasContent = true
			}
			if part.FunctionCall != nil {
				b, _ := json.Marshal(part.FunctionCall.Args)
				asstMsg.ToolCalls = append(asstMsg.ToolCalls, openai.ToolCall{
					ID:   part.FunctionCall.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(b),
					},
				})
				asstHasContent = true
			}

			if part.FunctionResponse != nil {
				b, _ := json.Marshal(part.FunctionResponse.Response)
				toolResponses = append(toolResponses, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					ToolCallID: part.FunctionResponse.ID,
					Content:    string(b),
				})
			}
		}

		if asstHasContent {
			asstMsg.Content = text
			messages = append(messages, asstMsg)
		}

		if len(toolResponses) > 0 {
			messages = append(messages, toolResponses...)
		}
	}

	if len(messages) == 0 {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "Handle the requests as specified in the System Instruction.",
		})
	}

	return cleanOpenAIMessages(messages)
}

// cleanOpenAIMessages ensures tool calls and responses form valid sequences to avoid 400 Bad Requests.
func cleanOpenAIMessages(original []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	var cleaned []openai.ChatCompletionMessage
	validToolCallIDs := make(map[string]bool)

	for _, msg := range original {
		if msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
			// Record valid tool calls emitted by assistant
			for _, tc := range msg.ToolCalls {
				validToolCallIDs[tc.ID] = true
			}
			cleaned = append(cleaned, msg)
		} else if msg.Role == openai.ChatMessageRoleTool {
			// Only keep tool responses for tool calls we've seen from an assistant in history
			if validToolCallIDs[msg.ToolCallID] {
				cleaned = append(cleaned, msg)
				// Ensure we only mark it answered once? For simplicity, just allow it.
			} else {
				log.Printf("Warning: Dropping orphan tool response with ID %s", msg.ToolCallID)
			}
		} else {
			cleaned = append(cleaned, msg)
		}
	}

	// Ensure all tool calls at the end of history have matching responses, or prune un-answered ones IF they are trailing.
	// Actually, if the last message in history is an assistant with tool_calls, but no subsequent tool message exists,
	// OpenAI might reject it. However, in our flow, ADK intercepts pending tool calls before making a new history request.
	// So any assistant message in history SHOULD have a response. If it doesn't, we should prune the `tool_calls` array for that message.

	answeredToolCalls := make(map[string]bool)
	for _, msg := range cleaned {
		if msg.Role == openai.ChatMessageRoleTool {
			answeredToolCalls[msg.ToolCallID] = true
		}
	}

	var finalCleaned []openai.ChatCompletionMessage
	for _, msg := range cleaned {
		if msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
			var prunedCalls []openai.ToolCall
			for _, tc := range msg.ToolCalls {
				if answeredToolCalls[tc.ID] {
					prunedCalls = append(prunedCalls, tc)
				} else {
					log.Printf("Warning: Pruning un-answered tool call %s from history", tc.ID)
				}
			}
			msg.ToolCalls = prunedCalls
		}

		// If an assistant message ends up with no content and no tool calls, drop it entirely to avoid 400 Bad Request
		if msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) == 0 && msg.Content == "" {
			continue
		}

		finalCleaned = append(finalCleaned, msg)
	}

	return finalCleaned
}

func (m *openaiModel) toLLMResponse(resp *openai.ChatCompletionResponse) *model.LLMResponse {
	llmResp := &model.LLMResponse{
		Content: &genai.Content{
			Role:  "model",
			Parts: make([]*genai.Part, 0),
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		if choice.Message.Content != "" {
			llmResp.Content.Parts = append(llmResp.Content.Parts, &genai.Part{
				Text: choice.Message.Content,
			})
		}

		for _, tc := range choice.Message.ToolCalls {
			if tc.Type == openai.ToolTypeFunction {
				var args map[string]any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					log.Printf("Warning: failed to unmarshal tool args: %v", err)
				}
				llmResp.Content.Parts = append(llmResp.Content.Parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						ID:   tc.ID,
						Name: tc.Function.Name,
						Args: args,
					},
				})
			}
		}

		switch choice.FinishReason {
		case openai.FinishReasonStop:
			llmResp.FinishReason = genai.FinishReasonStop
		case openai.FinishReasonLength:
			llmResp.FinishReason = genai.FinishReasonMaxTokens
		case openai.FinishReasonToolCalls:
			// Unspecified/pending tool call state.
		}
	}

	if resp.Usage.TotalTokens > 0 {
		llmResp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(resp.Usage.PromptTokens),
			CandidatesTokenCount: int32(resp.Usage.CompletionTokens),
			TotalTokenCount:      int32(resp.Usage.TotalTokens),
		}
	}

	return llmResp
}

func (m *openaiModel) toLLMResponseChunk(resp *openai.ChatCompletionStreamResponse) *model.LLMResponse {
	llmResp := &model.LLMResponse{
		Content: &genai.Content{
			Role:  "model",
			Parts: make([]*genai.Part, 0),
		},
		Partial: true,
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]

		if choice.Delta.Content != "" {
			llmResp.Content.Parts = append(llmResp.Content.Parts, &genai.Part{
				Text: choice.Delta.Content,
			})
		}
		for _, tc := range choice.Delta.ToolCalls {
			if tc.Type == openai.ToolTypeFunction {
				var args map[string]any
				if tc.Function.Arguments != "" {
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
						// Stream might send partial arguments
					}
				}
				llmResp.Content.Parts = append(llmResp.Content.Parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						ID:   tc.ID,
						Name: tc.Function.Name,
						Args: args,
					},
				})
			}
		}

		if choice.FinishReason != "" && choice.FinishReason != "null" {
			llmResp.TurnComplete = true
			llmResp.Partial = false
			switch choice.FinishReason {
			case openai.FinishReasonStop:
				llmResp.FinishReason = genai.FinishReasonStop
			case openai.FinishReasonLength:
				llmResp.FinishReason = genai.FinishReasonMaxTokens
			}
		}
	}

	return llmResp
}
