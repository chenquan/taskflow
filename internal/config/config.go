package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/chenquan/specflow/internal/domain"
	"github.com/chenquan/specflow/internal/fsx"
	"gopkg.in/yaml.v3"
)

var repoName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
var changeID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ValidateTaskID(taskID string) error {
	if taskID == "" || taskID == "." || taskID == ".." || filepath.IsAbs(taskID) || filepath.Base(taskID) != taskID || strings.ContainsAny(taskID, `/\\`) {
		return fmt.Errorf("task ID must be one safe path component")
	}
	return nil
}

func Path(tasksRoot, taskID string) string { return filepath.Join(tasksRoot, taskID, "specflow.yaml") }
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
	if err = Validate(&t); err != nil {
		return domain.Task{}, err
	}
	return t, nil
}
func Marshal(t domain.Task) ([]byte, error) { return yaml.Marshal(t) }
func Validate(t *domain.Task) error {
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
	root, err := fsx.CanonicalExisting(t.Task.Root)
	if err != nil {
		return fmt.Errorf("task.root: %w", err)
	}
	t.Task.Root = root
	if len(t.Repositories) == 0 {
		return fmt.Errorf("at least one repository is required")
	}
	seen := map[string]bool{}
	primary := false
	worktrees := filepath.Join(root, "worktrees")
	for i := range t.Repositories {
		r := &t.Repositories[i]
		if !repoName.MatchString(r.Name) {
			return fmt.Errorf("invalid repository name %q", r.Name)
		}
		if seen[r.Name] {
			return fmt.Errorf("duplicate repository %q", r.Name)
		}
		seen[r.Name] = true
		if r.Name == t.Primary {
			primary = true
		}
		source, err := fsx.CanonicalExisting(r.Source)
		if err != nil {
			return fmt.Errorf("repository %s source: %w", r.Name, err)
		}
		info, err := os.Stat(source)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("repository %s source is not a directory", r.Name)
		}
		output, err := exec.Command("git", "-C", source, "rev-parse", "--is-inside-work-tree").Output()
		if err != nil || strings.TrimSpace(string(output)) != "true" {
			return fmt.Errorf("repository %s source is not a Git worktree", r.Name)
		}
		r.Source = source
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
		if !fsx.Within(worktrees, target) {
			return fmt.Errorf("repository %s worktree escapes task worktrees", r.Name)
		}
		rel, err := filepath.Rel(root, target)
		if err != nil {
			return err
		}
		r.Worktree = rel
		if r.Base == "" {
			r.Base = "HEAD"
		}
		if r.Branch == "" {
			r.Branch = "feature/" + strings.ToLower(t.Task.ID)
		}
		if r.Change == "" {
			r.Change = ChangeID(t.Task.ID, r.Name)
		}
		if !changeID.MatchString(r.Change) {
			return fmt.Errorf("repository %s has invalid change ID %q", r.Name, r.Change)
		}
		for _, c := range r.Checks {
			if c.Name == "" || c.Executable == "" {
				return fmt.Errorf("repository %s has invalid check", r.Name)
			}
			if c.Timeout != "" {
				if _, err := time.ParseDuration(c.Timeout); err != nil {
					return fmt.Errorf("repository %s check %s has invalid timeout: %w", r.Name, c.Name, err)
				}
			}
		}
	}
	if !primary {
		return fmt.Errorf("primary repository %q is not configured", t.Primary)
	}
	for _, r := range t.Repositories {
		for _, dep := range r.DependsOn {
			if !seen[dep] {
				return fmt.Errorf("repository %s depends on unknown repository %s", r.Name, dep)
			}
		}
	}
	if err := checkAcyclic(t.Repositories); err != nil {
		return err
	}
	return nil
}
func ChangeID(taskID, name string) string {
	s := strings.ToLower(taskID + "-" + name)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
func checkAcyclic(repos []domain.Repository) error {
	by := map[string]domain.Repository{}
	for _, r := range repos {
		by[r.Name] = r
	}
	visiting := map[string]bool{}
	done := map[string]bool{}
	var visit func(string) error
	visit = func(n string) error {
		if visiting[n] {
			return fmt.Errorf("repository dependency cycle includes %s", n)
		}
		if done[n] {
			return nil
		}
		visiting[n] = true
		for _, d := range by[n].DependsOn {
			if err := visit(d); err != nil {
				return err
			}
		}
		visiting[n] = false
		done[n] = true
		return nil
	}
	names := make([]string, 0, len(by))
	for n := range by {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if err := visit(n); err != nil {
			return err
		}
	}
	return nil
}
