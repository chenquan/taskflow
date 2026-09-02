package ownership

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/fsx"
)

const Version = 1

// Manifest records worktrees that Taskflow created itself. A configured
// worktree without an entry is intentionally not considered safe to delete:
// it may have been created manually and merely adopted by create.
type Manifest struct {
	Version   int        `json:"version"`
	TaskID    string     `json:"taskID"`
	Worktrees []Worktree `json:"worktrees"`
}

type Worktree struct {
	Repository string   `json:"repository"`
	Source     string   `json:"source"`
	CommonDir  string   `json:"commonDir"`
	Branch     string   `json:"branch"`
	Target     string   `json:"target"`
	Overlay    *Overlay `json:"overlay,omitempty"`
}

// Overlay is a narrow creation-time snapshot. It is optional so manifests
// written by older Taskflow versions remain valid and continue to represent
// worktrees with no tracked overlay history.
type Overlay struct {
	Source     string               `json:"source"`
	Target     string               `json:"target"`
	Paths      []string             `json:"paths"`
	Files      []domain.OverlayFile `json:"files"`
	TotalBytes int64                `json:"totalBytes"`
	Status     string               `json:"status"` // pending or complete
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
		if err := validateOverlay(worktree); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manifest) Add(worktree Worktree) {
	for index := range m.Worktrees {
		current := &m.Worktrees[index]
		if current.Source == worktree.Source && current.Branch == worktree.Branch && current.Target == worktree.Target {
			current.Repository = worktree.Repository
			current.CommonDir = worktree.CommonDir
			current.Overlay = worktree.Overlay
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

func validateOverlay(worktree Worktree) error {
	overlay := worktree.Overlay
	if overlay == nil {
		return nil
	}
	if overlay.Source == "" || overlay.Target == "" {
		return fmt.Errorf("overlay ownership entry for %q is incomplete", worktree.Repository)
	}
	if overlay.Status != "pending" && overlay.Status != "complete" {
		return fmt.Errorf("overlay ownership entry for %q has unsupported status %q", worktree.Repository, overlay.Status)
	}
	if overlay.Source != worktree.Source || overlay.Target != worktree.Target {
		return fmt.Errorf("overlay ownership entry for %q has stale source or target identity", worktree.Repository)
	}
	seenPaths := map[string]struct{}{}
	for _, path := range overlay.Paths {
		if err := validateRelativePath(path); err != nil {
			return fmt.Errorf("overlay ownership entry for %q: %w", worktree.Repository, err)
		}
		if _, exists := seenPaths[path]; exists {
			return fmt.Errorf("overlay ownership entry for %q contains duplicate path %q", worktree.Repository, path)
		}
		seenPaths[path] = struct{}{}
	}
	seenFiles := map[string]struct{}{}
	var total int64
	for _, file := range overlay.Files {
		if err := validateRelativePath(file.Path); err != nil {
			return fmt.Errorf("overlay ownership entry for %q: %w", worktree.Repository, err)
		}
		if _, exists := seenFiles[file.Path]; exists {
			return fmt.Errorf("overlay ownership entry for %q contains duplicate file %q", worktree.Repository, file.Path)
		}
		seenFiles[file.Path] = struct{}{}
		if file.Size < 0 || file.Mode > 0777 || len(file.SHA256) != 64 {
			return fmt.Errorf("overlay ownership entry for %q contains invalid file metadata", worktree.Repository)
		}
		if _, err := hex.DecodeString(file.SHA256); err != nil {
			return fmt.Errorf("overlay ownership entry for %q contains invalid file metadata", worktree.Repository)
		}
		total += file.Size
	}
	if total != overlay.TotalBytes {
		return fmt.Errorf("overlay ownership entry for %q has invalid total size", worktree.Repository)
	}
	return nil
}

func validateRelativePath(path string) error {
	portable := strings.ReplaceAll(path, `\`, "/")
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) || strings.HasPrefix(portable, "/") || isWindowsAbsolute(portable) {
		return fmt.Errorf("overlay path %q must be source-relative", path)
	}
	clean := pathpkg.Clean(portable)
	if clean != path {
		return fmt.Errorf("overlay path %q is not normalized", path)
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("overlay path %q escapes its source", path)
	}
	for _, component := range strings.FieldsFunc(clean, func(r rune) bool { return r == '/' || r == '\\' }) {
		if component == ".git" {
			return fmt.Errorf("overlay path %q may not contain .git", path)
		}
	}
	return nil
}

func isWindowsAbsolute(path string) bool {
	return len(path) >= 2 && ((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) && path[1] == ':'
}
