package skills

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RegistryEntry is a lightweight index record stored in registry.json.
type RegistryEntry struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	InstalledAt string `json:"installed_at"`
}

// RegistryIndex is the contents of registry.json.
type RegistryIndex struct {
	Skills []RegistryEntry `json:"skills"`
}

// Add upserts an entry by name.
func (idx *RegistryIndex) Add(entry RegistryEntry) {
	for i, e := range idx.Skills {
		if e.Name == entry.Name {
			idx.Skills[i] = entry
			return
		}
	}
	idx.Skills = append(idx.Skills, entry)
}

// Remove deletes an entry by name.
func (idx *RegistryIndex) Remove(name string) {
	for i, e := range idx.Skills {
		if e.Name == name {
			idx.Skills = append(idx.Skills[:i], idx.Skills[i+1:]...)
			return
		}
	}
}

// LoadRegistryIndex reads registry.json from the skills directory.
// Returns an empty index if the file doesn't exist.
func LoadRegistryIndex(dir string) (RegistryIndex, error) {
	path := filepath.Join(dir, "registry.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RegistryIndex{}, nil
		}
		return RegistryIndex{}, fmt.Errorf("read registry: %w", err)
	}
	var idx RegistryIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return RegistryIndex{}, fmt.Errorf("parse registry: %w", err)
	}
	return idx, nil
}

// SaveRegistryIndex writes registry.json atomically via temp+rename.
func SaveRegistryIndex(dir string, idx RegistryIndex) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	path := filepath.Join(dir, "registry.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write registry tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename registry: %w", err)
	}
	return nil
}

// Install copies a skill from sourcePath into the loader's skills directory,
// parses it, checks requirements, and updates the registry index.
func (l *Loader) Install(sourcePath string) (*Skill, error) {
	// Read and parse the SKILL.md from source.
	mdPath := filepath.Join(sourcePath, "SKILL.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, fmt.Errorf("read source SKILL.md: %w", err)
	}

	skill, err := ParseSkillMD(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse SKILL.md: %w", err)
	}

	// Determine name.
	if skill.Name == "" {
		skill.Name = filepath.Base(sourcePath)
	}

	// Copy source directory to skills dir.
	destDir := filepath.Join(l.Dir, skill.Name)
	if err := os.MkdirAll(l.Dir, 0755); err != nil {
		return nil, fmt.Errorf("create skills dir: %w", err)
	}

	// Remove existing if present (overwrite).
	os.RemoveAll(destDir)

	if err := copyDir(sourcePath, destDir); err != nil {
		return nil, fmt.Errorf("copy skill: %w", err)
	}

	skill.Dir = destDir
	checkRequirements(&skill)

	// Update in-memory state.
	// Remove old entry if exists.
	for i, s := range l.Skills {
		if s.Name == skill.Name {
			l.Skills = append(l.Skills[:i], l.Skills[i+1:]...)
			break
		}
	}
	l.Skills = append(l.Skills, skill)
	l.byName[skill.Name] = &l.Skills[len(l.Skills)-1]

	// Update registry index.
	idx, _ := LoadRegistryIndex(l.Dir)
	idx.Add(RegistryEntry{
		Name:        skill.Name,
		Version:     skill.Version,
		Description: skill.Description,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	})
	SaveRegistryIndex(l.Dir, idx)

	return l.byName[skill.Name], nil
}

// Remove deletes a skill by name from disk, memory, and the registry index.
func (l *Loader) Remove(name string) error {
	sk := l.byName[name]
	if sk == nil {
		return fmt.Errorf("skill not found: %s", name)
	}

	// Remove from disk.
	if sk.Dir != "" {
		if err := os.RemoveAll(sk.Dir); err != nil {
			return fmt.Errorf("remove skill dir: %w", err)
		}
	}

	// Remove from in-memory state.
	delete(l.byName, name)
	for i, s := range l.Skills {
		if s.Name == name {
			l.Skills = append(l.Skills[:i], l.Skills[i+1:]...)
			break
		}
	}

	// Update registry index.
	idx, _ := LoadRegistryIndex(l.Dir)
	idx.Remove(name)
	SaveRegistryIndex(l.Dir, idx)

	return nil
}

// copyDir recursively copies a directory tree using only stdlib.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
