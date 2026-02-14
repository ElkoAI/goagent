package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Skill represents a loaded skill parsed from a SKILL.md file.
type Skill struct {
	// Frontmatter fields
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Requires    Requires `json:"requires"`

	// Derived
	Dir  string `json:"dir"`  // absolute path to skill directory
	Body string `json:"body"` // markdown body (instructions for the agent)

	// Status after requirement check
	Ready    bool     `json:"ready"`
	Missing  []string `json:"missing,omitempty"` // missing bins/env
}

// Requires declares what a skill needs on the system.
type Requires struct {
	Bins []string `json:"bins,omitempty"` // required binaries (gh, curl, ffmpeg)
	Env  []string `json:"env,omitempty"`  // required environment variables
}

// Loader discovers and manages skills from a directory.
type Loader struct {
	Dir    string // e.g. ~/.goagent/skills
	Skills []Skill
}

// NewLoader creates a loader for the given skills directory.
func NewLoader(dataDir string) *Loader {
	return &Loader{Dir: filepath.Join(dataDir, "skills")}
}

// LoadAll scans the skills directory and loads every SKILL.md found.
func (l *Loader) LoadAll() error {
	l.Skills = nil

	entries, err := os.ReadDir(l.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no skills directory yet — that's fine
		}
		return fmt.Errorf("read skills dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(l.Dir, entry.Name())
		mdPath := filepath.Join(skillDir, "SKILL.md")

		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue // no SKILL.md — skip
		}

		skill, err := ParseSkillMD(string(data))
		if err != nil {
			continue // malformed — skip
		}

		skill.Dir = skillDir
		if skill.Name == "" {
			skill.Name = entry.Name() // fallback to directory name
		}

		checkRequirements(&skill)
		l.Skills = append(l.Skills, skill)
	}

	return nil
}

// Ready returns only skills whose requirements are satisfied.
func (l *Loader) Ready() []Skill {
	var ready []Skill
	for _, s := range l.Skills {
		if s.Ready {
			ready = append(ready, s)
		}
	}
	return ready
}

// Get returns a skill by name, or nil if not found.
func (l *Loader) Get(name string) *Skill {
	for i := range l.Skills {
		if l.Skills[i].Name == name {
			return &l.Skills[i]
		}
	}
	return nil
}

// BuildPrompt concatenates the markdown bodies of all ready skills
// into a single string for system prompt injection.
func (l *Loader) BuildPrompt() string {
	ready := l.Ready()
	if len(ready) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n\n---\n# Available Skills\n")
	for _, s := range ready {
		fmt.Fprintf(&b, "\n## Skill: %s\n", s.Name)
		if s.Description != "" {
			fmt.Fprintf(&b, "%s\n", s.Description)
		}
		b.WriteString("\n")
		b.WriteString(s.Body)
		b.WriteString("\n")
	}
	return b.String()
}

// ParseSkillMD parses a SKILL.md file into a Skill struct.
// Format: YAML frontmatter (--- delimited) + markdown body.
// Supports OpenClaw's single-line frontmatter constraint.
func ParseSkillMD(content string) (Skill, error) {
	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return Skill{}, err
	}

	skill := Skill{Body: body}
	meta := parseFrontmatterLines(frontmatter)

	skill.Name = meta["name"]
	skill.Description = meta["description"]

	// Parse metadata JSON (OpenClaw format):
	// metadata: {"openclaw":{"requires":{"bins":["gh"],"env":["GITHUB_TOKEN"]}}}
	if raw, ok := meta["metadata"]; ok {
		var md struct {
			OpenClaw struct {
				Requires Requires `json:"requires"`
			} `json:"openclaw"`
		}
		if err := json.Unmarshal([]byte(raw), &md); err == nil {
			skill.Requires = md.OpenClaw.Requires
		}
	}

	return skill, nil
}

// splitFrontmatter splits --- delimited YAML frontmatter from the markdown body.
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		// No frontmatter — entire content is the body.
		return "", content, nil
	}

	// Find closing ---.
	rest := content[3:]
	rest = strings.TrimLeft(rest, " \t")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", content, fmt.Errorf("unclosed frontmatter (missing closing ---)")
	}

	frontmatter = rest[:idx]
	body = strings.TrimSpace(rest[idx+4:])
	return frontmatter, body, nil
}

// parseFrontmatterLines parses simple key: value lines from YAML frontmatter.
// Values are kept as raw strings (JSON values are not parsed here).
func parseFrontmatterLines(fm string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		result[key] = val
	}
	return result
}

// checkRequirements validates that a skill's bins and env vars are present.
func checkRequirements(s *Skill) {
	s.Ready = true
	s.Missing = nil

	for _, bin := range s.Requires.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			s.Ready = false
			s.Missing = append(s.Missing, "bin:"+bin)
		}
	}

	for _, env := range s.Requires.Env {
		if os.Getenv(env) == "" {
			s.Ready = false
			s.Missing = append(s.Missing, "env:"+env)
		}
	}
}
