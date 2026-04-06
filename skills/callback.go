package skills

import (
	"fmt"
	"strings"

	"github.com/osiloke/adk-go-essentials/observability"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// InjectActiveSkills is a BeforeModelCallback that injects all currently activated skills from session state.
func InjectActiveSkills(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	return injectActiveSkillsWithFilter(ctx, req, "")
}

// InjectActiveSkillsWithFilter creates a BeforeModelCallback that injects active skills filtered by directory.
// The filterDir parameter specifies which skills directory to include (e.g., ".product_designer/use_case_skills").
// If filterDir is empty, all active skills are injected.
func InjectActiveSkillsWithFilter(filterDir string) func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	return func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
		return injectActiveSkillsWithFilter(ctx, req, filterDir)
	}
}

// injectActiveSkillsWithFilter is the internal implementation that handles skill injection with optional filtering.
func injectActiveSkillsWithFilter(ctx agent.CallbackContext, req *model.LLMRequest, filterDir string) (*model.LLMResponse, error) {
	state := ctx.State()
	if state == nil {
		return nil, nil
	}

	var activeSkills []string
	var skippedSkills []string
	for k, v := range state.All() {
		// Check if this is a skill key (starts with "active_skill_")
		if after, ok := strings.CutPrefix(k, "active_skill_"); ok {
			skillName := after

			// Check if we need to filter by directory
			if filterDir != "" {
				// Get the directory for this specific skill
				dirKey := fmt.Sprintf("skill_dir_%s", skillName)
				skillDir, _ := state.Get(dirKey)
				if skillDir != nil {
					if dirStr, ok := skillDir.(string); ok {
						if strings.Contains(dirStr, filterDir) {
							skillText := fmt.Sprintf("---\nACTIVE SKILL: %s ---\n%v\n", skillName, v)
							activeSkills = append(activeSkills, skillText)
						} else {
							skippedSkills = append(skippedSkills, fmt.Sprintf("%s (dir=%s, filter=%s)", skillName, dirStr, filterDir))
						}
					}
				} else {
					skippedSkills = append(skippedSkills, fmt.Sprintf("%s (no dir key: %s)", skillName, dirKey))
				}
			} else {
				// No filter, include all skills
				skillText := fmt.Sprintf("---\nACTIVE SKILL: %s ---\n%v\n", skillName, v)
				activeSkills = append(activeSkills, skillText)
			}
		}
	}

	if len(skippedSkills) > 0 {
		observability.Log.Debugf("🔍 [Skills] Skipped %d skills due to filter: [%s]", len(skippedSkills), strings.Join(skippedSkills, ", "))
	}

	if len(activeSkills) == 0 {
		return nil, nil
	}

	header := "ACTIVE AGENT SKILLS"
	if filterDir != "" {
		if strings.Contains(filterDir, "use_case_skills") {
			header = "ACTIVE USE CASE SKILLS"
		} else if strings.Contains(filterDir, "frontend") {
			header = "ACTIVE FRONTEND SKILLS"
		} else if strings.Contains(filterDir, "backend") {
			header = "ACTIVE BACKEND SKILLS"
		}
	}

	injection := fmt.Sprintf("\n\n### %s\nYou have previously activated the following skills. Follow their instructions strictly:\n%s", header, strings.Join(activeSkills, "\n"))

	if req.Config.SystemInstruction == nil {
		req.Config.SystemInstruction = &genai.Content{
			Role:  "system",
			Parts: []*genai.Part{{Text: injection}},
		}
	} else {
		if len(req.Config.SystemInstruction.Parts) > 0 {
			req.Config.SystemInstruction.Parts[0].Text += injection
		} else {
			req.Config.SystemInstruction.Parts = append(req.Config.SystemInstruction.Parts, &genai.Part{Text: injection})
		}
	}

	return nil, nil
}
