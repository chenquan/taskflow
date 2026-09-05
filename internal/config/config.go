package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/fsx"
	"gopkg.in/yaml.v3"
)

var repoName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

func ValidateTaskID(taskID string) error {
	if taskID == "" || taskID == "." || taskID == ".." || filepath.IsAbs(taskID) || filepath.Base(taskID) != taskID || strings.ContainsAny(taskID, `/\\`) {
		return fmt.Errorf("task ID must be one safe path component")
	}
	return nil
}

func Path(tasksRoot, taskID string) string { return filepath.Join(tasksRoot, taskID, "taskflow.yaml") }

func Load(path string) (domain.Task, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return domain.Task{}, err
	}
	var t domain.Task
	d := yaml.NewDecoder(bytes.NewReader(b))
	d.KnownFields(true)
	if err = d.Decode(&t); err != nil {
		return domain.Task{}, fmt.Errorf("decode %s: %w", path, err)
	}
	t.Version = domain.ConfigVersion
	t.Task.Root, err = filepath.Abs(filepath.Dir(path))
	if err != nil {
		return domain.Task{}, fmt.Errorf("resolve task root for %s: %w", path, err)
	}
	if err = Validate(&t); err != nil {
		return domain.Task{}, err
	}
	return t, nil
}

func Marshal(t domain.Task) ([]byte, error) { return yaml.Marshal(t) }

func Validate(t *domain.Task) error {
	if t.Version == 0 {
		t.Version = domain.ConfigVersion
	}
	if t.Version != domain.ConfigVersion {
		return fmt.Errorf("unsupported config version %d", t.Version)
	}
	if strings.TrimSpace(t.Task.ID) == "" {
		return fmt.Errorf("task.id is required")
	}
	if err := ValidateTaskID(t.Task.ID); err != nil {
		return err
	}
	if strings.TrimSpace(t.Task.Root) == "" {
		return fmt.Errorf("task.root is required")
	}
	root, err := fsx.CanonicalManaged(t.Task.Root)
	if err != nil {
		return fmt.Errorf("task.root: %w", err)
	}
	t.Task.Root = root
	if len(t.Repositories) == 0 {
		return fmt.Errorf("at least one repository is required")
	}
	seenNames := map[string]bool{}
	seenTargets := map[string]string{}
	worktreesRoot := filepath.Join(root, "worktrees")
	for i := range t.Repositories {
		r := &t.Repositories[i]
		if !repoName.MatchString(r.Name) {
			return fmt.Errorf("invalid repository name %q", r.Name)
		}
		if seenNames[r.Name] {
			return fmt.Errorf("duplicate repository %q", r.Name)
		}
		seenNames[r.Name] = true
		source, err := fsx.CanonicalExisting(r.Source)
		if err != nil {
			return fmt.Errorf("repository %s source: %w", r.Name, err)
		}
		info, err := os.Stat(source)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("repository %s source is not a directory", r.Name)
		}
		r.Source = source
		if r.Base == "" {
			r.Base = "HEAD"
		}
		if r.Branch == "" {
			r.Branch = "feature/" + strings.ToLower(t.Task.ID)
		}
		if r.Worktree == "" {
			r.Worktree = filepath.Join("worktrees", r.Name)
		}
		target := r.Worktree
		if !filepath.IsAbs(target) {
			target = filepath.Join(root, target)
		}
		target, err = fsx.CanonicalManaged(target)
		if err != nil {
			return fmt.Errorf("repository %s worktree: %w", r.Name, err)
		}
		if !fsx.Within(worktreesRoot, target) {
			return fmt.Errorf("repository %s worktree escapes task worktrees", r.Name)
		}
		key := filepath.Clean(target)
		if other, exists := seenTargets[key]; exists {
			return fmt.Errorf("repositories %s and %s use the same worktree target", other, r.Name)
		}
		seenTargets[key] = r.Name
		rel, err := filepath.Rel(root, target)
		if err != nil {
			return err
		}
		r.Worktree = rel
	}
	return nil
}
