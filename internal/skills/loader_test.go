package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleSkillMD = `---
name: github
description: GitHub API integration for PRs, issues, and repos
metadata: {"openclaw":{"requires":{"bins":["ls"],"env":["HOME"]}}}
---

# GitHub Skill

When the user asks about pull requests, use ` + "`gh pr list`" + `.
When asked to create an issue, use ` + "`gh issue create`" + `.
`

const minimalSkillMD = `---
name: simple
description: A simple skill with no requirements
---

Just do the thing.
`

const noFrontmatterSkillMD = `# Raw Skill

This skill has no frontmatter at all.
`

func TestParseSkillMD(t *testing.T) {
	skill, err := ParseSkillMD(sampleSkillMD)
	if err != nil {
		t.Fatalf("ParseSkillMD error: %v", err)
	}

	if skill.Name != "github" {
		t.Errorf("Name = %q, want github", skill.Name)
	}
	if skill.Description != "GitHub API integration for PRs, issues, and repos" {
		t.Errorf("Description = %q", skill.Description)
	}
	if len(skill.Requires.Bins) != 1 || skill.Requires.Bins[0] != "ls" {
		t.Errorf("Requires.Bins = %v, want [ls]", skill.Requires.Bins)
	}
	if len(skill.Requires.Env) != 1 || skill.Requires.Env[0] != "HOME" {
		t.Errorf("Requires.Env = %v, want [HOME]", skill.Requires.Env)
	}
	if !strings.Contains(skill.Body, "GitHub Skill") {
		t.Errorf("Body doesn't contain 'GitHub Skill': %q", skill.Body)
	}
}

func TestParseSkillMD_Minimal(t *testing.T) {
	skill, err := ParseSkillMD(minimalSkillMD)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if skill.Name != "simple" {
		t.Errorf("Name = %q", skill.Name)
	}
	if len(skill.Requires.Bins) != 0 {
		t.Errorf("Requires.Bins should be empty, got %v", skill.Requires.Bins)
	}
	if !strings.Contains(skill.Body, "Just do the thing") {
		t.Errorf("Body = %q", skill.Body)
	}
}

func TestParseSkillMD_NoFrontmatter(t *testing.T) {
	skill, err := ParseSkillMD(noFrontmatterSkillMD)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if skill.Name != "" {
		t.Errorf("Name should be empty, got %q", skill.Name)
	}
	if !strings.Contains(skill.Body, "Raw Skill") {
		t.Errorf("Body = %q", skill.Body)
	}
}

func TestCheckRequirements_AllPresent(t *testing.T) {
	s := Skill{
		Requires: Requires{
			Bins: []string{"ls"},   // exists on any Linux system
			Env:  []string{"HOME"}, // always set
		},
	}
	checkRequirements(&s)
	if !s.Ready {
		t.Errorf("skill should be ready, missing: %v", s.Missing)
	}
}

func TestCheckRequirements_MissingBin(t *testing.T) {
	s := Skill{
		Requires: Requires{
			Bins: []string{"nonexistent_binary_xyz_123"},
		},
	}
	checkRequirements(&s)
	if s.Ready {
		t.Error("skill should not be ready (missing binary)")
	}
	if len(s.Missing) != 1 || s.Missing[0] != "bin:nonexistent_binary_xyz_123" {
		t.Errorf("Missing = %v", s.Missing)
	}
}

func TestCheckRequirements_MissingEnv(t *testing.T) {
	s := Skill{
		Requires: Requires{
			Env: []string{"GOAGENT_NONEXISTENT_VAR_XYZ"},
		},
	}
	checkRequirements(&s)
	if s.Ready {
		t.Error("skill should not be ready (missing env)")
	}
	if len(s.Missing) != 1 || s.Missing[0] != "env:GOAGENT_NONEXISTENT_VAR_XYZ" {
		t.Errorf("Missing = %v", s.Missing)
	}
}

func TestLoaderLoadAll(t *testing.T) {
	// Create temp skills directory with two skills.
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// Skill 1: github (requirements satisfied)
	githubDir := filepath.Join(skillsDir, "github")
	os.MkdirAll(githubDir, 0755)
	os.WriteFile(filepath.Join(githubDir, "SKILL.md"), []byte(sampleSkillMD), 0644)

	// Skill 2: minimal
	simpleDir := filepath.Join(skillsDir, "simple")
	os.MkdirAll(simpleDir, 0755)
	os.WriteFile(filepath.Join(simpleDir, "SKILL.md"), []byte(minimalSkillMD), 0644)

	// Skill 3: directory without SKILL.md (should be skipped)
	emptyDir := filepath.Join(skillsDir, "empty")
	os.MkdirAll(emptyDir, 0755)

	loader := &Loader{Dir: skillsDir}
	if err := loader.LoadAll(); err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}

	if len(loader.Skills) != 2 {
		t.Fatalf("loaded %d skills, want 2", len(loader.Skills))
	}

	// Both should be ready (ls and HOME exist).
	ready := loader.Ready()
	if len(ready) != 2 {
		t.Errorf("ready = %d, want 2", len(ready))
	}
}

func TestLoaderLoadAll_NoDir(t *testing.T) {
	loader := &Loader{Dir: "/tmp/goagent-nonexistent-dir-xyz"}
	if err := loader.LoadAll(); err != nil {
		t.Fatalf("LoadAll should not error on missing dir, got: %v", err)
	}
	if len(loader.Skills) != 0 {
		t.Errorf("should have 0 skills")
	}
}

func TestLoaderGet(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	githubDir := filepath.Join(skillsDir, "github")
	os.MkdirAll(githubDir, 0755)
	os.WriteFile(filepath.Join(githubDir, "SKILL.md"), []byte(sampleSkillMD), 0644)

	loader := &Loader{Dir: skillsDir}
	loader.LoadAll()

	s := loader.Get("github")
	if s == nil {
		t.Fatal("Get(github) returned nil")
	}
	if s.Name != "github" {
		t.Errorf("Name = %q", s.Name)
	}

	if loader.Get("nonexistent") != nil {
		t.Error("Get(nonexistent) should return nil")
	}
}

func TestLoaderBuildPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	githubDir := filepath.Join(skillsDir, "github")
	os.MkdirAll(githubDir, 0755)
	os.WriteFile(filepath.Join(githubDir, "SKILL.md"), []byte(sampleSkillMD), 0644)

	loader := &Loader{Dir: skillsDir}
	loader.LoadAll()

	prompt := loader.BuildPrompt()
	if !strings.Contains(prompt, "Available Skills") {
		t.Error("prompt should contain 'Available Skills' header")
	}
	if !strings.Contains(prompt, "## Skill: github") {
		t.Error("prompt should contain skill heading")
	}
	if !strings.Contains(prompt, "gh pr list") {
		t.Error("prompt should contain skill body instructions")
	}
}

func TestLoaderBuildPrompt_Empty(t *testing.T) {
	loader := &Loader{Dir: "/tmp/goagent-nonexistent-dir-xyz"}
	loader.LoadAll()

	prompt := loader.BuildPrompt()
	if prompt != "" {
		t.Errorf("empty loader should produce empty prompt, got: %q", prompt)
	}
}

func TestLoaderNameFallback(t *testing.T) {
	// Skill with no name in frontmatter should use directory name.
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	mySkillDir := filepath.Join(skillsDir, "my-cool-skill")
	os.MkdirAll(mySkillDir, 0755)
	os.WriteFile(filepath.Join(mySkillDir, "SKILL.md"), []byte(noFrontmatterSkillMD), 0644)

	loader := &Loader{Dir: skillsDir}
	loader.LoadAll()

	if len(loader.Skills) != 1 {
		t.Fatalf("loaded %d skills, want 1", len(loader.Skills))
	}
	if loader.Skills[0].Name != "my-cool-skill" {
		t.Errorf("Name = %q, want my-cool-skill", loader.Skills[0].Name)
	}
}
