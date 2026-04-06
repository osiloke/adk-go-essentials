package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a reusable agent capability with instructions and optional reference documents.
type Skill struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Content     string            `yaml:"-"` // Skill instructions
	Dir         string            `yaml:"-"` // Directory containing the skill
	References  map[string]string `yaml:"-"` // Reference documents (e.g., tool-schemas.md)
}

// Registry manages skill loading and lookup.
type Registry struct {
	skills map[string]Skill
}

// NewRegistry creates a new skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]Skill),
	}
}

// LoadFromDir scans the directory recursively for SKILL.md files and loads them.
func (r *Registry) LoadFromDir(root string) error {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil // No skills directory, that's fine
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(d.Name(), "SKILL.md") {
			return r.loadSkill(path)
		}
		return nil
	})
}

func (r *Registry) loadSkill(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Parse Frontmatter
	content := string(data)
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		idx := strings.Index(content, "---")
		if idx == -1 {
			return fmt.Errorf("skill at %s missing frontmatter", path)
		}
		content = content[idx:]
	}

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return fmt.Errorf("skill at %s has malformed frontmatter", path)
	}

	frontmatter := parts[1]
	body := parts[2]

	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return fmt.Errorf("failed to parse frontmatter for %s: %w", path, err)
	}

	if skill.Name == "" {
		return fmt.Errorf("skill at %s missing 'name' in frontmatter", path)
	}

	skill.Content = strings.TrimSpace(body)
	skill.Dir = filepath.Dir(path)
	skill.References = loadSkillReferences(skill.Dir)

	r.skills[skill.Name] = skill
	return nil
}

// loadSkillReferences loads all reference documents from a skill's references/ directory.
func loadSkillReferences(skillDir string) map[string]string {
	references := make(map[string]string)
	referencesDir := filepath.Join(skillDir, "references")

	if _, err := os.Stat(referencesDir); os.IsNotExist(err) {
		return references
	}

	entries, err := os.ReadDir(referencesDir)
	if err != nil {
		return references
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		refPath := filepath.Join(referencesDir, entry.Name())
		content, err := os.ReadFile(refPath)
		if err != nil {
			continue
		}

		references[entry.Name()] = string(content)
	}

	return references
}

// List returns all registered skills.
func (r *Registry) List() []Skill {
	var list []Skill
	for _, s := range r.skills {
		list = append(list, s)
	}
	return list
}

// Get returns a skill by name.
func (r *Registry) Get(name string) (*Skill, error) {
	if s, ok := r.skills[name]; ok {
		return &s, nil
	}
	return nil, fmt.Errorf("skill %q not found", name)
}

// GetReference retrieves a specific reference document from a skill.
func (r *Registry) GetReference(skillName, refName string) (string, error) {
	skill, err := r.Get(skillName)
	if err != nil {
		return "", err
	}

	if content, ok := skill.References[refName]; ok {
		return content, nil
	}
	return "", fmt.Errorf("reference %q not found in skill %q", refName, skillName)
}

// ListReferences returns all reference document names for a skill.
func (r *Registry) ListReferences(skillName string) ([]string, error) {
	skill, err := r.Get(skillName)
	if err != nil {
		return nil, err
	}

	var refs []string
	for name := range skill.References {
		refs = append(refs, name)
	}
	return refs, nil
}
