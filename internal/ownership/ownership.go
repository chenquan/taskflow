package ownership

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenquan/taskflow/internal/fsx"
)

const Version = 1

// Manifest records worktrees that Taskflow created itself. A configured
// worktree without an entry is intentionally not considered safe to delete:
// it may have been created manually and merely adopted by open/create.
type Manifest struct {
	Version   int        `json:"version"`
	TaskID    string     `json:"taskID"`
	Worktrees []Worktree `json:"worktrees"`
}

type Worktree struct {
	Repository string `json:"repository"`
	Source     string `json:"source"`
	CommonDir  string `json:"commonDir"`
	Branch     string `json:"branch"`
	Target     string `json:"target"`
}

func Path(taskRoot string) string {
	return filepath.Join(taskRoot, ".taskflow", "ownership.json")
}

func New(taskID string) Manifest {
	return Manifest{Version: Version, TaskID: taskID, Worktrees: []Worktree{}}
}

func Load(taskRoot string) (Manifest, bool, error) {
	raw, err := os.ReadFile(Path(taskRoot))
	if os.IsNotExist(err) {
		return Manifest{}, false, nil
	}
	if err != nil {
		return Manifest{}, false, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, true, fmt.Errorf("decode %s: %w", Path(taskRoot), err)
	}
	if err := Validate(manifest); err != nil {
		return Manifest{}, true, fmt.Errorf("validate %s: %w", Path(taskRoot), err)
	}
	return manifest, true, nil
}

func Validate(manifest Manifest) error {
	if manifest.Version != Version {
		return fmt.Errorf("unsupported ownership version %d", manifest.Version)
	}
	if manifest.TaskID == "" {
		return fmt.Errorf("taskID is required")
	}
	seen := map[string]bool{}
	for _, worktree := range manifest.Worktrees {
		if worktree.Repository == "" || worktree.Source == "" || worktree.CommonDir == "" || worktree.Branch == "" || worktree.Target == "" {
			return fmt.Errorf("ownership entry for %q is incomplete", worktree.Repository)
		}
		key := worktree.Source + "\x00" + worktree.Branch + "\x00" + worktree.Target
		if seen[key] {
			return fmt.Errorf("duplicate ownership entry for %q", worktree.Repository)
		}
		seen[key] = true
	}
	return nil
}

func (m *Manifest) Add(worktree Worktree) {
	for index := range m.Worktrees {
		current := &m.Worktrees[index]
		if current.Source == worktree.Source && current.Branch == worktree.Branch && current.Target == worktree.Target {
			current.Repository = worktree.Repository
			current.CommonDir = worktree.CommonDir
			return
		}
	}
	m.Worktrees = append(m.Worktrees, worktree)
}

func Save(taskRoot string, manifest Manifest) error {
	if err := Validate(manifest); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return fsx.AtomicWrite(Path(taskRoot), raw, 0644)
}
