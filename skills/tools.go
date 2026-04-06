package skills

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// NewListSkillsTool creates a tool to list available skills.
func NewListSkillsTool(r *Registry) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "list_skills",
		Description: "Lists all available agent skills with their descriptions.",
	}, func(ctx tool.Context, _ struct{}) (string, error) {
		skills := r.List()
		if len(skills) == 0 {
			return "No skills available.", nil
		}
		var sb strings.Builder
		sb.WriteString("Available Skills:\n")
		for _, s := range skills {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
		}
		return sb.String(), nil
	})
}

// ActivateSkillArgs defines the arguments for activating a skill.
type ActivateSkillArgs struct {
	Name string `json:"name" description:"The name of the skill to activate"`
}

// NewActivateSkillTool creates a tool to retrieve skill instructions and persist them to state.
func NewActivateSkillTool(r *Registry) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "activate_skill",
		Description: "Activates a skill by name. This persists its instructions to the session context and returns them.",
	}, func(ctx tool.Context, args ActivateSkillArgs) (string, error) {
		skill, err := r.Get(args.Name)
		if err != nil {
			return fmt.Sprintf("Error: %v", err), nil
		}

		// Persist to session state so callbacks can inject it into every prompt
		state := ctx.State()
		if state != nil {
			key := fmt.Sprintf("active_skill_%s", skill.Name)
			_ = state.Set(key, skill.Content)
			// Store skill directory with a separate key for filtering
			_ = state.Set(fmt.Sprintf("skill_dir_%s", skill.Name), skill.Dir)
		}

		return fmt.Sprintf("### SKILL ACTIVATED: %s\n\nInstructions:\n%s", skill.Name, skill.Content), nil
	})
}

// ListReferencesArgs defines arguments for listing references in a skill.
type ListReferencesArgs struct {
	SkillName string `json:"skill_name" description:"The name of the skill to list references for"`
}

// NewListReferencesTool creates a tool to list reference documents for a skill.
func NewListReferencesTool(r *Registry) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "list_references",
		Description: "Lists all reference documents (e.g., tool-schemas.md, design-mappings.md) available for a skill.",
	}, func(ctx tool.Context, args ListReferencesArgs) (string, error) {
		refs, err := r.ListReferences(args.SkillName)
		if err != nil {
			return fmt.Sprintf("Error: %v", err), nil
		}
		if len(refs) == 0 {
			return fmt.Sprintf("No reference documents found for skill %q.", args.SkillName), nil
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Reference documents for skill %q:\n", args.SkillName))
		for _, ref := range refs {
			sb.WriteString(fmt.Sprintf("- %s\n", ref))
		}
		return sb.String(), nil
	})
}

// LoadReferenceArgs defines arguments for loading a reference document.
type LoadReferenceArgs struct {
	SkillName string `json:"skill_name" description:"The name of the skill containing the reference"`
	RefName   string `json:"ref_name" description:"The name of the reference document (e.g., 'tool-schemas.md')"`
}

// NewLoadReferenceTool creates a tool to load a specific reference document from a skill.
func NewLoadReferenceTool(r *Registry) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "load_reference",
		Description: "Loads a specific reference document from a skill's references/ directory. Use this to get tool schemas, design mappings, or other reference materials.",
	}, func(ctx tool.Context, args LoadReferenceArgs) (string, error) {
		content, err := r.GetReference(args.SkillName, args.RefName)
		if err != nil {
			return fmt.Sprintf("Error: %v", err), nil
		}
		return fmt.Sprintf("### Reference: %s/%s\n\n%s", args.SkillName, args.RefName, content), nil
	})
}

// RunSkillScriptArgs defines the arguments for running a script within a skill.
type RunSkillScriptArgs struct {
	SkillName  string `json:"skill_name" description:"The name of the skill containing the script"`
	ScriptName string `json:"script_name" description:"The name of the script file to run (e.g., 'process.py')"`
	Args       string `json:"args" description:"Space-separated arguments for the script"`
}

// NewRunSkillScriptTool creates a tool to execute scripts bundled with a skill.
func NewRunSkillScriptTool(r *Registry) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "run_skill_script",
		Description: "Executes a script found in a skill's directory.",
	}, func(ctx tool.Context, args RunSkillScriptArgs) (string, error) {
		skill, err := r.Get(args.SkillName)
		if err != nil {
			return fmt.Sprintf("Error finding skill: %v", err), nil
		}

		scriptPath := filepath.Join(skill.Dir, args.ScriptName)

		// Simple execution logic. For production, this should be sandboxed.
		var cmd *exec.Cmd
		if strings.HasSuffix(args.ScriptName, ".py") {
			cmd = exec.Command("python3", append([]string{scriptPath}, strings.Fields(args.Args)...)...)
		} else if strings.HasSuffix(args.ScriptName, ".sh") {
			cmd = exec.Command("bash", append([]string{scriptPath}, strings.Fields(args.Args)...)...)
		} else {
			cmd = exec.Command(scriptPath, strings.Fields(args.Args)...)
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("Error executing script: %v\nOutput: %s", err, string(output)), nil
		}

		return fmt.Sprintf("Script Output:\n%s", string(output)), nil
	})
}

// FormatToolSchema formats a tool schema as JSON for display.
func FormatToolSchema(name string, schema map[string]any) string {
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting schema: %v", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Tool: `%s`\n\n", name))
	sb.WriteString("**Schema**:\n```json\n")
	sb.WriteString(string(data))
	sb.WriteString("\n```\n")
	return sb.String()
}
