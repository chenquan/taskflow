// Package skills contains the built-in agent skills distributed by SpecFlow.
package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Files contains the complete built-in skill directories.
//
//go:embed all:specflow
var Files embed.FS

type Target struct {
	Tool string
	Root string
}

type InstallResult struct {
	Tool   string   `json:"tool"`
	Root   string   `json:"root"`
	Skills []string `json:"skills"`
}

type replacement struct {
	target string
	stage  string
	backup string
}

func Names() ([]string, error) {
	entries, err := fs.ReadDir(Files, ".")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := fs.Stat(Files, filepath.ToSlash(filepath.Join(entry.Name(), "SKILL.md"))); err == nil {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Install copies every embedded skill into each tool skill root. It rejects all
// existing skill directories unless force is true, and stages every replacement
// before modifying a target.
func Install(targets []Target, force bool) ([]InstallResult, error) {
	names, err := Names()
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one installation target is required")
	}
	for _, target := range targets {
		if target.Tool == "" || target.Root == "" {
			return nil, fmt.Errorf("installation target must include tool and root")
		}
		for _, name := range names {
			path := filepath.Join(target.Root, name)
			info, statErr := os.Lstat(path)
			if os.IsNotExist(statErr) {
				continue
			}
			if statErr != nil {
				return nil, fmt.Errorf("inspect %s skill %q: %w", target.Tool, name, statErr)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("refusing symbolic-link skill target %q", path)
			}
			if !force {
				return nil, fmt.Errorf("skill %q already exists at %s; rerun with --force to replace it", name, path)
			}
			if !info.IsDir() {
				return nil, fmt.Errorf("skill target %q is not a directory", path)
			}
		}
	}

	replacements := make([]replacement, 0, len(targets)*len(names))
	for _, target := range targets {
		if err := os.MkdirAll(target.Root, 0755); err != nil {
			cleanupStages(replacements)
			return nil, fmt.Errorf("create %s skill root: %w", target.Tool, err)
		}
		for _, name := range names {
			stage, err := os.MkdirTemp(target.Root, ".specflow-skill-")
			if err != nil {
				cleanupStages(replacements)
				return nil, fmt.Errorf("stage %s skill %q: %w", target.Tool, name, err)
			}
			if err := copySkill(name, stage); err != nil {
				os.RemoveAll(stage)
				cleanupStages(replacements)
				return nil, err
			}
			replacements = append(replacements, replacement{target: filepath.Join(target.Root, name), stage: stage})
		}
	}

	for i := range replacements {
		item := &replacements[i]
		if _, err := os.Lstat(item.target); err == nil {
			item.backup = item.target + ".specflow-backup"
			if err := os.Rename(item.target, item.backup); err != nil {
				rollback(replacements)
				return nil, fmt.Errorf("backup existing skill %q: %w", item.target, err)
			}
		}
		if err := os.Rename(item.stage, item.target); err != nil {
			rollback(replacements)
			return nil, fmt.Errorf("install skill %q: %w", item.target, err)
		}
		item.stage = ""
	}
	for _, item := range replacements {
		if item.backup != "" {
			if err := os.RemoveAll(item.backup); err != nil {
				return nil, fmt.Errorf("remove replaced skill backup %q: %w", item.backup, err)
			}
		}
	}

	result := make([]InstallResult, 0, len(targets))
	for _, target := range targets {
		result = append(result, InstallResult{Tool: target.Tool, Root: target.Root, Skills: names})
	}
	return result, nil
}

func copySkill(name, destination string) error {
	return fs.WalkDir(Files, name, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(name, filepath.FromSlash(path))
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		content, err := Files.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0644)
	})
}

func cleanupStages(replacements []replacement) {
	for _, item := range replacements {
		if item.stage != "" {
			_ = os.RemoveAll(item.stage)
		}
	}
}

func rollback(replacements []replacement) {
	for i := len(replacements) - 1; i >= 0; i-- {
		item := replacements[i]
		if item.stage != "" {
			_ = os.RemoveAll(item.stage)
		}
		if item.backup == "" {
			_ = os.RemoveAll(item.target)
			continue
		}
		_ = os.RemoveAll(item.target)
		_ = os.Rename(item.backup, item.target)
	}
}
