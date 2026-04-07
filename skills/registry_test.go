package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if r.skills == nil {
		t.Error("registry skills map should not be nil")
	}
}

func TestRegistry_LoadFromDir_NonExistent(t *testing.T) {
	r := NewRegistry()
	err := r.LoadFromDir("/nonexistent/path")
	if err != nil {
		t.Fatalf("LoadFromDir should not error for non-existent dir: %v", err)
	}
}

func TestRegistry_LoadFromDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry()
	err := r.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir error = %v", err)
	}
	if len(r.List()) != 0 {
		t.Errorf("expected 0 skills, got %d", len(r.List()))
	}
}

func TestRegistry_LoadFromDir_SingleSkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "test_skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: test-skill
description: A test skill
---
This is the skill content.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	if err := r.LoadFromDir(dir); err != nil {
		t.Fatalf("LoadFromDir error = %v", err)
	}

	skills := r.List()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Name != "test-skill" {
		t.Errorf("expected name 'test-skill', got %q", s.Name)
	}
	if s.Description != "A test skill" {
		t.Errorf("expected description 'A test skill', got %q", s.Description)
	}
	if !strings.Contains(s.Content, "This is the skill content") {
		t.Errorf("expected content to contain skill text, got %q", s.Content)
	}
}

func TestRegistry_Get_Existing(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my_skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: my-skill
description: My skill
---
Skill body.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	if err := r.LoadFromDir(dir); err != nil {
		t.Fatal(err)
	}

	s, err := r.Get("my-skill")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if s.Name != "my-skill" {
		t.Errorf("expected name 'my-skill', got %q", s.Name)
	}
}

func TestRegistry_Get_NonExistent(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent skill, got nil")
	}
}

func TestRegistry_LoadFromDir_MissingFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "bad_skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := "No frontmatter here."
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	err := r.LoadFromDir(dir)
	if err == nil {
		t.Fatal("expected error for missing frontmatter, got nil")
	}
}

func TestRegistry_LoadFromDir_MissingName(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "noname_skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
description: Has description but no name
---
Content.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	err := r.LoadFromDir(dir)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestRegistry_LoadFromDir_MalformedFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "malformed")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: test
description: desc
---
only one closing dash`
	// Actually this is valid frontmatter with only 2 dashes at the start.
	// Let me create truly malformed:
	content = `name: test
description: desc`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	err := r.LoadFromDir(dir)
	if err == nil {
		t.Fatal("expected error for malformed frontmatter, got nil")
	}
}

func TestRegistry_LoadFromDir_WithReferences(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "ref_skill")
	refDir := filepath.Join(skillDir, "references")
	if err := os.MkdirAll(refDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: ref-skill
description: A skill with references
---
Skill content.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refDir, "schema.md"), []byte("# Schema"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	if err := r.LoadFromDir(dir); err != nil {
		t.Fatal(err)
	}

	s, err := r.Get("ref-skill")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.References) != 1 {
		t.Fatalf("expected 1 reference, got %d", len(s.References))
	}
	if _, ok := s.References["schema.md"]; !ok {
		t.Error("expected reference schema.md")
	}
}

func TestRegistry_ListReferences(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "list_refs_skill")
	refDir := filepath.Join(skillDir, "references")
	if err := os.MkdirAll(refDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: list-refs
description: Test
---
Body.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"schema.md", "design.md"} {
		if err := os.WriteFile(filepath.Join(refDir, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	r := NewRegistry()
	if err := r.LoadFromDir(dir); err != nil {
		t.Fatal(err)
	}

	refs, err := r.ListReferences("list-refs")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 references, got %d", len(refs))
	}
}

func TestRegistry_GetReference_NonExistentSkill(t *testing.T) {
	r := NewRegistry()
	_, err := r.GetReference("no-such-skill", "schema.md")
	if err == nil {
		t.Fatal("expected error for nonexistent skill, got nil")
	}
}

func TestRegistry_GetReference_NonExistentRef(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "no_ref_skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: no-ref-skill
description: Test
---
Body.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	if err := r.LoadFromDir(dir); err != nil {
		t.Fatal(err)
	}

	_, err := r.GetReference("no-ref-skill", "missing.md")
	if err == nil {
		t.Fatal("expected error for nonexistent reference, got nil")
	}
}

func TestRegistry_LoadFromDir_NestedDirectories(t *testing.T) {
	dir := t.TempDir()
	skillDir1 := filepath.Join(dir, "frontend", "react")
	skillDir2 := filepath.Join(dir, "backend", "api")
	if err := os.MkdirAll(skillDir1, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(skillDir2, 0755); err != nil {
		t.Fatal(err)
	}

	for i, path := range []string{skillDir1, skillDir2} {
		names := []string{"react-skill", "api-skill"}
		content := `---
name: ` + names[i] + `
description: Desc
---
Content.
`
		if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	r := NewRegistry()
	if err := r.LoadFromDir(dir); err != nil {
		t.Fatal(err)
	}

	skills := r.List()
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills from nested dirs, got %d", len(skills))
	}
}

func TestRegistry_LoadFromDir_SkillDirStored(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dir_test_skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: dir-test
description: Test
---
Body.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	if err := r.LoadFromDir(dir); err != nil {
		t.Fatal(err)
	}

	s, err := r.Get("dir-test")
	if err != nil {
		t.Fatal(err)
	}
	if s.Dir != skillDir {
		t.Errorf("expected Dir %q, got %q", skillDir, s.Dir)
	}
}

func TestRegistry_LoadFromDir_CaseInsensitiveSkillFileName(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "case_skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: case-test
description: Test
---
Body.
`
	// Test with lowercase skill.md - should be matched by strings.EqualFold
	if err := os.WriteFile(filepath.Join(skillDir, "skill.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	if err := r.LoadFromDir(dir); err != nil {
		t.Fatal(err)
	}

	skills := r.List()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill from lowercase skill.md, got %d", len(skills))
	}
}

func TestRegistry_ListReferences_NonExistentSkill(t *testing.T) {
	r := NewRegistry()
	_, err := r.ListReferences("no-skill")
	if err == nil {
		t.Fatal("expected error for nonexistent skill, got nil")
	}
}

func TestRegistry_List_NoSkills(t *testing.T) {
	r := NewRegistry()
	skills := r.List()
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestRegistry_LoadFromDir_SkillWithYAMLSpecialChars(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "yaml_special")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Content with colons and special YAML chars in description
	content := `---
name: yaml-special
description: "A skill with: colons, [brackets], and {braces}"
---
Body with --- dashes.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRegistry()
	if err := r.LoadFromDir(dir); err != nil {
		t.Fatal(err)
	}

	s, err := r.Get("yaml-special")
	if err != nil {
		t.Fatal(err)
	}
	if s.Description == "" {
		t.Error("expected description to be parsed despite special chars")
	}
}
