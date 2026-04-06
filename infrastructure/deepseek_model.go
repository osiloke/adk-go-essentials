package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"log"

	"github.com/cohesion-org/deepseek-go"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type deepseekModel struct {
	client *deepseek.Client
	name   string
}

// NewModel creates a new model.LLM backed by the cohesion-org deepseek client.
func NewModel(apiKey string, modelName string) (model.LLM, error) {
	client := deepseek.NewClient(apiKey)

	// Access HTTPClient if exposed, though cohesion-org/deepseek-go doesn't easily expose it.
	// We'll set it manually if possible, or wait to see if it causes a build error.
	// Actually, looking at deepseek-go, it usually has client.HTTPClient or we can just hope the OpenAI wrapper using the deepseek URL is more stable.
	// For now, let's just use the OpenAI wrapper for DeepSeek which we KNOW we can configure.
	// Wait, we already use NewOpenAIModel for DeepSeek when DEEPSEEK_OPENAI_KEY is set.
	// The problem in the logs shown is: Post "https://api.deepseek.com/chat/completions": context canceled
	// This was triggered through the openai wrapper! ("failed to call openai model:")

	return &deepseekModel{
		client: client,
		name:   modelName,
	}, nil
}

func (m *deepseekModel) Name() string {
	return m.name
}

func (m *deepseekModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	if stream {
		return m.generateStream(ctx, req)
	}

	return func(yield func(*model.LLMResponse, error) bool) {
		resp, err := m.generate(ctx, req)
		yield(resp, err)
	}
}

func (m *deepseekModel) generate(ctx context.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
	chatReq := m.toDeepseekRequest(req)
	resp, err := m.client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call model: %w", err)
	}
	return m.toLLMResponse(resp), nil
}

func (m *deepseekModel) generateStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		chatReq := m.toDeepseekStreamRequest(req)

		stream, err := m.client.CreateChatCompletionStream(ctx, chatReq)
		if err != nil {
			yield(nil, fmt.Errorf("failed to start stream: %w", err))
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

			llmResp := m.toLLMResponseChunk(chunk)
			if !yield(llmResp, nil) {
				return // Consumer stopped
			}
		}
	}
}

func (m *deepseekModel) toDeepseekRequest(req *model.LLMRequest) *deepseek.ChatCompletionRequest {
	chatReq := &deepseek.ChatCompletionRequest{
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
			// DeepSeek API has a max limit of 8192 tokens
			maxTokens := int(req.Config.MaxOutputTokens)
			if maxTokens > 8192 {
				maxTokens = 8192
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
			chatReq.ResponseFormat = &deepseek.ResponseFormat{Type: "json_object"}
		}

		chatReq.Tools = m.convertTools(req.Config.Tools)
	}

	return chatReq
}

func (m *deepseekModel) toDeepseekStreamRequest(req *model.LLMRequest) *deepseek.StreamChatCompletionRequest {
	chatReq := &deepseek.StreamChatCompletionRequest{
		Model:  m.name,
		Stream: true,
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
			// DeepSeek API has a max limit of 8192 tokens
			maxTokens := int(req.Config.MaxOutputTokens)
			if maxTokens > 8192 {
				maxTokens = 8192
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
			chatReq.ResponseFormat = &deepseek.ResponseFormat{Type: "json_object"}
		}

		chatReq.Tools = m.convertTools(req.Config.Tools)
	}

	return chatReq
}

func (m *deepseekModel) convertTools(tools []*genai.Tool) []deepseek.Tool {
	if len(tools) == 0 {
		return nil
	}
	var res []deepseek.Tool
	for _, t := range tools {
		if t.FunctionDeclarations != nil {
			for _, fd := range t.FunctionDeclarations {
				b, _ := json.Marshal(fd.Parameters)
				var params map[string]interface{}
				json.Unmarshal(b, &params)

				var props map[string]interface{}
				if p, ok := params["properties"]; ok && p != nil {
					if pm, ok := p.(map[string]interface{}); ok {
						props = pm
					}
				}

				var reqs []string
				if r, ok := params["required"]; ok && r != nil {
					reqs = convertToStringSlice(r)
				}

				res = append(res, deepseek.Tool{
					Type: "function",
					Function: deepseek.Function{
						Name:        fd.Name,
						Description: fd.Description,
						Parameters: &deepseek.FunctionParameters{
							Type:       "object",
							Properties: props,
							Required:   reqs,
						},
					},
				})
			}
		}
	}
	return res
}

func convertToStringSlice(val interface{}) []string {
	if val == nil {
		return nil
	}
	if arr, ok := val.([]interface{}); ok {
		var res []string
		for _, v := range arr {
			if str, ok := v.(string); ok {
				res = append(res, str)
			}
		}
		return res
	}
	return nil
}

func (m *deepseekModel) convertMessages(req *model.LLMRequest) []deepseek.ChatCompletionMessage {
	var messages []deepseek.ChatCompletionMessage

	// Add system instruction if present
	if req.Config != nil && req.Config.SystemInstruction != nil && len(req.Config.SystemInstruction.Parts) > 0 {
		text := ""
		for _, part := range req.Config.SystemInstruction.Parts {
			if part.Text != "" {
				text += part.Text
			}
		}
		messages = append(messages, deepseek.ChatCompletionMessage{
			Role:    deepseek.ChatMessageRoleSystem,
			Content: text,
		})
	}

	// Add contents
	for _, content := range req.Contents {
		role := deepseek.ChatMessageRoleUser
		if content.Role == "model" {
			role = deepseek.ChatMessageRoleAssistant
		}

		// Because a single genai.Content block can encapsulate both an assistant's thought process
		// (including deciding on FunctionCalls) AND the subsequent `FunctionResponse` role blocks,
		// we must split them into distinct DeepSeek messages.

		var asstMsg deepseek.ChatCompletionMessage
		asstMsg.Role = role
		asstHasContent := false

		var toolResponses []deepseek.ChatCompletionMessage

		// Handle Parts
		text := ""
		for _, part := range content.Parts {
			if part.Text != "" {
				text += part.Text
				asstHasContent = true
			}
			if part.FunctionCall != nil {
				// The assistant invoked a tool. This stringifies the args so DeepSeek understands what was called.
				b, _ := json.Marshal(part.FunctionCall.Args)
				asstMsg.ToolCalls = append(asstMsg.ToolCalls, deepseek.ToolCall{
					ID:   part.FunctionCall.ID,
					Type: "function",
					Function: deepseek.ToolCallFunction{
						Name:      part.FunctionCall.Name,
						Arguments: string(b),
					},
				})
				asstHasContent = true
			}

			if part.FunctionResponse != nil {
				// The result of a tool call. DeepSeek mandates a dedicated message block with Role="tool"
				b, _ := json.Marshal(part.FunctionResponse.Response)
				toolResponses = append(toolResponses, deepseek.ChatCompletionMessage{
					Role:       deepseek.ChatMessageRoleTool,
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
		messages = append(messages, deepseek.ChatCompletionMessage{
			Role:    deepseek.ChatMessageRoleUser,
			Content: "Handle the requests as specified in the System Instruction.",
		})
	}

	return cleanDeepseekMessages(messages)
}

// cleanDeepseekMessages ensures tool calls and responses form valid sequences to avoid 400 Bad Requests.
func cleanDeepseekMessages(original []deepseek.ChatCompletionMessage) []deepseek.ChatCompletionMessage {
	var cleaned []deepseek.ChatCompletionMessage
	validToolCallIDs := make(map[string]bool)

	for _, msg := range original {
		if msg.Role == deepseek.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				validToolCallIDs[tc.ID] = true
			}
			cleaned = append(cleaned, msg)
		} else if msg.Role == deepseek.ChatMessageRoleTool {
			if validToolCallIDs[msg.ToolCallID] {
				cleaned = append(cleaned, msg)
			} else {
				log.Printf("Warning: Dropping orphan tool response with ID %s", msg.ToolCallID)
			}
		} else {
			cleaned = append(cleaned, msg)
		}
	}

	answeredToolCalls := make(map[string]bool)
	for _, msg := range cleaned {
		if msg.Role == deepseek.ChatMessageRoleTool {
			answeredToolCalls[msg.ToolCallID] = true
		}
	}

	var finalCleaned []deepseek.ChatCompletionMessage
	for _, msg := range cleaned {
		if msg.Role == deepseek.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
			var prunedCalls []deepseek.ToolCall
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
		if msg.Role == deepseek.ChatMessageRoleAssistant && len(msg.ToolCalls) == 0 && msg.Content == "" {
			continue
		}

		finalCleaned = append(finalCleaned, msg)
	}

	return finalCleaned
}

func (m *deepseekModel) toLLMResponse(resp *deepseek.ChatCompletionResponse) *model.LLMResponse {
	llmResp := &model.LLMResponse{
		Content: &genai.Content{
			Role:  "model",
			Parts: make([]*genai.Part, 0),
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		if choice.Message.ReasoningContent != "" {
			llmResp.Content.Parts = append(llmResp.Content.Parts, &genai.Part{
				Text:    choice.Message.ReasoningContent,
				Thought: true,
			})
		}
		if choice.Message.Content != "" {
			llmResp.Content.Parts = append(llmResp.Content.Parts, &genai.Part{
				Text: choice.Message.Content,
			})
		}

		for _, tc := range choice.Message.ToolCalls {
			if tc.Type == "function" {
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
		case "stop":
			llmResp.FinishReason = genai.FinishReasonStop
		case "length":
			llmResp.FinishReason = genai.FinishReasonMaxTokens
		case "tool_calls":
			// Leave as unspecified, it's technically a stop for ADK
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

func (m *deepseekModel) toLLMResponseChunk(resp *deepseek.StreamChatCompletionResponse) *model.LLMResponse {
	llmResp := &model.LLMResponse{
		Content: &genai.Content{
			Role:  "model",
			Parts: make([]*genai.Part, 0),
		},
		Partial: true,
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]

		if choice.Delta.ReasoningContent != "" {
			llmResp.Content.Parts = append(llmResp.Content.Parts, &genai.Part{
				Text:    choice.Delta.ReasoningContent,
				Thought: true,
			})
		}
		if choice.Delta.Content != "" {
			llmResp.Content.Parts = append(llmResp.Content.Parts, &genai.Part{
				Text: choice.Delta.Content,
			})
		}
		for _, tc := range choice.Delta.ToolCalls {
			if tc.Type == "function" {
				var args map[string]any
				if tc.Function.Arguments != "" {
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
						// Stream might send partial arguments, which we can't unmarshal yet easily.
						// DeepSeek streams full tool_calls usually or we might have to accumulate them.
						// For ADK streaming tool calls, we assume it's sent whole or we skip tool streaming chunks.
						// The cohesive deepseek lib doesn't easily accumulate stream toolcalls yet.
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
			case "stop":
				llmResp.FinishReason = genai.FinishReasonStop
			case "length":
				llmResp.FinishReason = genai.FinishReasonMaxTokens
			}
		}
	}

	return llmResp
}
