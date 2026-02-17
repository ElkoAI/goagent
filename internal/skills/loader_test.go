package skills

import (
	"encoding/json"
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

const fullFieldsSkillMD = `---
name: deploy
description: Deploy to production
version: 2.1.0
license: MIT
homepage: https://example.com/deploy
tools: Bash, Read, Edit
user-invocable: false
metadata: {"openclaw":{"always":true,"emoji":"🚀","os":["linux","darwin"],"primaryEnv":"DEPLOY_KEY","requires":{"bins":["ssh"],"anyBins":["rsync","scp"],"env":["DEPLOY_HOST"]}}}
---

# Deploy Skill

Deploy stuff.
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
	// Default user-invocable should be true.
	if !skill.UserInvocable {
		t.Error("UserInvocable should default to true")
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

func TestParseSkillMD_AllFields(t *testing.T) {
	skill, err := ParseSkillMD(fullFieldsSkillMD)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if skill.Name != "deploy" {
		t.Errorf("Name = %q", skill.Name)
	}
	if skill.Version != "2.1.0" {
		t.Errorf("Version = %q, want 2.1.0", skill.Version)
	}
	if skill.License != "MIT" {
		t.Errorf("License = %q, want MIT", skill.License)
	}
	if skill.Homepage != "https://example.com/deploy" {
		t.Errorf("Homepage = %q", skill.Homepage)
	}
	if len(skill.Tools) != 3 || skill.Tools[0] != "Bash" || skill.Tools[1] != "Read" || skill.Tools[2] != "Edit" {
		t.Errorf("Tools = %v, want [Bash Read Edit]", skill.Tools)
	}
	if skill.UserInvocable {
		t.Error("UserInvocable should be false")
	}
}

func TestParseSkillMD_MetadataExtended(t *testing.T) {
	skill, err := ParseSkillMD(fullFieldsSkillMD)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if !skill.Always {
		t.Error("Always should be true")
	}
	if skill.Emoji != "🚀" {
		t.Errorf("Emoji = %q, want 🚀", skill.Emoji)
	}
	if len(skill.OS) != 2 || skill.OS[0] != "linux" || skill.OS[1] != "darwin" {
		t.Errorf("OS = %v, want [linux darwin]", skill.OS)
	}
	if skill.PrimaryEnv != "DEPLOY_KEY" {
		t.Errorf("PrimaryEnv = %q, want DEPLOY_KEY", skill.PrimaryEnv)
	}
	if len(skill.Requires.Bins) != 1 || skill.Requires.Bins[0] != "ssh" {
		t.Errorf("Requires.Bins = %v, want [ssh]", skill.Requires.Bins)
	}
	if len(skill.Requires.AnyBins) != 2 || skill.Requires.AnyBins[0] != "rsync" {
		t.Errorf("Requires.AnyBins = %v, want [rsync scp]", skill.Requires.AnyBins)
	}
	if len(skill.Requires.Env) != 1 || skill.Requires.Env[0] != "DEPLOY_HOST" {
		t.Errorf("Requires.Env = %v, want [DEPLOY_HOST]", skill.Requires.Env)
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

func TestCheckRequirements_AnyBins_Found(t *testing.T) {
	s := Skill{
		Requires: Requires{
			AnyBins: []string{"nonexistent_xyz", "ls"}, // ls exists
		},
	}
	checkRequirements(&s)
	if !s.Ready {
		t.Errorf("skill should be ready (ls exists), missing: %v", s.Missing)
	}
}

func TestCheckRequirements_AnyBins_None(t *testing.T) {
	s := Skill{
		Requires: Requires{
			AnyBins: []string{"nonexistent_a", "nonexistent_b"},
		},
	}
	checkRequirements(&s)
	if s.Ready {
		t.Error("skill should not be ready (no anyBins found)")
	}
	found := false
	for _, m := range s.Missing {
		if strings.HasPrefix(m, "anyBins:") {
			found = true
		}
	}
	if !found {
		t.Errorf("Missing should contain anyBins entry, got %v", s.Missing)
	}
}

func TestCheckRequirements_OS_Match(t *testing.T) {
	saved := currentOS
	defer func() { currentOS = saved }()
	currentOS = "linux"

	s := Skill{OS: []string{"linux", "darwin"}}
	checkRequirements(&s)
	if !s.Ready {
		t.Errorf("skill should be ready on linux, missing: %v", s.Missing)
	}
}

func TestCheckRequirements_OS_NoMatch(t *testing.T) {
	saved := currentOS
	defer func() { currentOS = saved }()
	currentOS = "windows"

	s := Skill{OS: []string{"linux", "darwin"}}
	checkRequirements(&s)
	if s.Ready {
		t.Error("skill should not be ready on windows")
	}
	if len(s.Missing) != 1 || s.Missing[0] != "os:windows" {
		t.Errorf("Missing = %v, want [os:windows]", s.Missing)
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

	loader := &Loader{Dir: skillsDir, byName: make(map[string]*Skill)}
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

	// Verify map index is populated.
	if loader.Get("github") == nil {
		t.Error("byName should contain github")
	}
	if loader.Get("simple") == nil {
		t.Error("byName should contain simple")
	}
}

func TestLoaderLoadAll_NoDir(t *testing.T) {
	loader := &Loader{Dir: "/tmp/goagent-nonexistent-dir-xyz", byName: make(map[string]*Skill)}
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

	loader := &Loader{Dir: skillsDir, byName: make(map[string]*Skill)}
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

	loader := &Loader{Dir: skillsDir, byName: make(map[string]*Skill)}
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
	loader := &Loader{Dir: "/tmp/goagent-nonexistent-dir-xyz", byName: make(map[string]*Skill)}
	loader.LoadAll()

	prompt := loader.BuildPrompt()
	if prompt != "" {
		t.Errorf("empty loader should produce empty prompt, got: %q", prompt)
	}
}

func TestLoaderBuildPrompt_Emoji(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	deployDir := filepath.Join(skillsDir, "deploy")
	os.MkdirAll(deployDir, 0755)
	os.WriteFile(filepath.Join(deployDir, "SKILL.md"), []byte(`---
name: deploy
description: Deploy stuff
metadata: {"openclaw":{"emoji":"🚀"}}
---

Deploy instructions.
`), 0644)

	loader := &Loader{Dir: skillsDir, byName: make(map[string]*Skill)}
	loader.LoadAll()

	prompt := loader.BuildPrompt()
	if !strings.Contains(prompt, "## Skill: 🚀 deploy") {
		t.Errorf("prompt should contain emoji in heading, got: %q", prompt)
	}
}

func TestLoaderBuildPrompt_UserInvocable(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")

	// Skill that is NOT user-invocable.
	bgDir := filepath.Join(skillsDir, "background")
	os.MkdirAll(bgDir, 0755)
	os.WriteFile(filepath.Join(bgDir, "SKILL.md"), []byte(`---
name: background
description: Background skill
user-invocable: false
---

Background instructions.
`), 0644)

	// Skill that IS user-invocable (default).
	visDir := filepath.Join(skillsDir, "visible")
	os.MkdirAll(visDir, 0755)
	os.WriteFile(filepath.Join(visDir, "SKILL.md"), []byte(`---
name: visible
description: Visible skill
---

Visible instructions.
`), 0644)

	loader := &Loader{Dir: skillsDir, byName: make(map[string]*Skill)}
	loader.LoadAll()

	prompt := loader.BuildPrompt()
	if strings.Contains(prompt, "background") {
		t.Error("non-invocable skill should not appear in prompt")
	}
	if !strings.Contains(prompt, "visible") {
		t.Error("invocable skill should appear in prompt")
	}
}

func TestLoaderNameFallback(t *testing.T) {
	// Skill with no name in frontmatter should use directory name.
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	mySkillDir := filepath.Join(skillsDir, "my-cool-skill")
	os.MkdirAll(mySkillDir, 0755)
	os.WriteFile(filepath.Join(mySkillDir, "SKILL.md"), []byte(noFrontmatterSkillMD), 0644)

	loader := &Loader{Dir: skillsDir, byName: make(map[string]*Skill)}
	loader.LoadAll()

	if len(loader.Skills) != 1 {
		t.Fatalf("loaded %d skills, want 1", len(loader.Skills))
	}
	if loader.Skills[0].Name != "my-cool-skill" {
		t.Errorf("Name = %q, want my-cool-skill", loader.Skills[0].Name)
	}
}

func TestLoaderInstall(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "installed")
	sourceDir := filepath.Join(tmpDir, "source", "my-skill")

	// Create source skill.
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte(`---
name: my-skill
description: A test skill
version: 1.0.0
---

Do things.
`), 0644)
	// Also include an extra file to verify recursive copy.
	os.MkdirAll(filepath.Join(sourceDir, "scripts"), 0755)
	os.WriteFile(filepath.Join(sourceDir, "scripts", "run.sh"), []byte("#!/bin/bash\necho hi"), 0755)

	loader := &Loader{Dir: skillsDir, byName: make(map[string]*Skill)}
	sk, err := loader.Install(sourceDir)
	if err != nil {
		t.Fatalf("Install error: %v", err)
	}

	if sk.Name != "my-skill" {
		t.Errorf("Name = %q", sk.Name)
	}
	if sk.Version != "1.0.0" {
		t.Errorf("Version = %q", sk.Version)
	}
	if !sk.Ready {
		t.Errorf("should be ready, missing: %v", sk.Missing)
	}

	// Verify in-memory.
	if loader.Get("my-skill") == nil {
		t.Error("Get should find installed skill")
	}
	if len(loader.Skills) != 1 {
		t.Errorf("Skills len = %d, want 1", len(loader.Skills))
	}

	// Verify files on disk.
	destMD := filepath.Join(skillsDir, "my-skill", "SKILL.md")
	if _, err := os.Stat(destMD); err != nil {
		t.Errorf("SKILL.md not copied: %v", err)
	}
	destScript := filepath.Join(skillsDir, "my-skill", "scripts", "run.sh")
	if _, err := os.Stat(destScript); err != nil {
		t.Errorf("scripts/run.sh not copied: %v", err)
	}

	// Verify registry.json.
	idx, err := LoadRegistryIndex(skillsDir)
	if err != nil {
		t.Fatalf("LoadRegistryIndex error: %v", err)
	}
	if len(idx.Skills) != 1 || idx.Skills[0].Name != "my-skill" {
		t.Errorf("registry index = %+v", idx.Skills)
	}
}

func TestLoaderRemove(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "installed")
	sourceDir := filepath.Join(tmpDir, "source", "removable")

	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte(minimalSkillMD), 0644)

	loader := &Loader{Dir: skillsDir, byName: make(map[string]*Skill)}
	loader.Install(sourceDir)

	if err := loader.Remove("simple"); err != nil {
		t.Fatalf("Remove error: %v", err)
	}

	if loader.Get("simple") != nil {
		t.Error("Get should return nil after remove")
	}
	if len(loader.Skills) != 0 {
		t.Errorf("Skills len = %d, want 0", len(loader.Skills))
	}

	// Verify disk cleanup.
	if _, err := os.Stat(filepath.Join(skillsDir, "simple")); !os.IsNotExist(err) {
		t.Error("skill directory should be removed from disk")
	}

	// Verify registry.json.
	idx, _ := LoadRegistryIndex(skillsDir)
	if len(idx.Skills) != 0 {
		t.Errorf("registry should be empty, got %+v", idx.Skills)
	}
}

func TestLoaderRemove_NotFound(t *testing.T) {
	loader := &Loader{Dir: t.TempDir(), byName: make(map[string]*Skill)}
	if err := loader.Remove("nonexistent"); err == nil {
		t.Error("Remove of nonexistent skill should error")
	}
}

func TestRegistryIndex_AddRemove(t *testing.T) {
	var idx RegistryIndex

	idx.Add(RegistryEntry{Name: "a", Version: "1.0.0"})
	idx.Add(RegistryEntry{Name: "b", Version: "2.0.0"})
	if len(idx.Skills) != 2 {
		t.Fatalf("len = %d, want 2", len(idx.Skills))
	}

	// Upsert existing.
	idx.Add(RegistryEntry{Name: "a", Version: "1.1.0"})
	if len(idx.Skills) != 2 {
		t.Fatalf("len after upsert = %d, want 2", len(idx.Skills))
	}
	if idx.Skills[0].Version != "1.1.0" {
		t.Errorf("version after upsert = %q, want 1.1.0", idx.Skills[0].Version)
	}

	idx.Remove("a")
	if len(idx.Skills) != 1 {
		t.Fatalf("len after remove = %d, want 1", len(idx.Skills))
	}
	if idx.Skills[0].Name != "b" {
		t.Errorf("remaining skill = %q, want b", idx.Skills[0].Name)
	}

	// Remove nonexistent is a no-op.
	idx.Remove("nonexistent")
	if len(idx.Skills) != 1 {
		t.Errorf("len after no-op remove = %d", len(idx.Skills))
	}
}

func TestRegistryIndex_Persistence(t *testing.T) {
	dir := t.TempDir()

	idx := RegistryIndex{}
	idx.Add(RegistryEntry{Name: "test", Version: "1.0.0", InstalledAt: "2026-01-01T00:00:00Z"})

	if err := SaveRegistryIndex(dir, idx); err != nil {
		t.Fatalf("SaveRegistryIndex error: %v", err)
	}

	loaded, err := LoadRegistryIndex(dir)
	if err != nil {
		t.Fatalf("LoadRegistryIndex error: %v", err)
	}
	if len(loaded.Skills) != 1 || loaded.Skills[0].Name != "test" {
		t.Errorf("loaded = %+v", loaded.Skills)
	}

	// Verify it's valid JSON.
	data, _ := os.ReadFile(filepath.Join(dir, "registry.json"))
	var check map[string]any
	if err := json.Unmarshal(data, &check); err != nil {
		t.Errorf("registry.json is not valid JSON: %v", err)
	}
}
