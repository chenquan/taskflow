package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenquan/taskflow/internal/domain"
)

func task(root string) domain.Task {
	return domain.Task{Version: domain.ConfigVersion, Task: domain.TaskInfo{ID: "A", Root: root}, Repositories: []domain.Repository{{Name: "one", Source: root}}}
}

func TestLoadRejectsRetiredAndUnknownFields(t *testing.T) {
	d := t.TempDir()
	base, err := Marshal(task(d))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(d, "taskflow.yaml")
	for _, field := range []string{"execution:", "depends_on:", "checks:", "unknown_field:"} {
		if err := os.WriteFile(path, append(base, []byte("\n"+field+" true\n")...), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), strings.TrimSuffix(field, ":")) {
			t.Fatalf("expected %s rejection, got %v", field, err)
		}
	}
}

func TestValidateDefaultsAndVersion(t *testing.T) {
	d := t.TempDir()
	v := task(d)
	v.Version = 0
	if err := Validate(&v); err != nil {
		t.Fatal(err)
	}
	if v.Version != domain.ConfigVersion || v.Repositories[0].Worktree != filepath.Join("worktrees", "one") || v.Repositories[0].Base != "HEAD" || v.Repositories[0].Branch != "feature/a" {
		t.Fatalf("unexpected defaults: %#v", v)
	}
}

func TestValidateDoesNotInspectGit(t *testing.T) {
	d := t.TempDir()
	v := task(d)
	if err := Validate(&v); err != nil {
		t.Fatalf("structural validation unexpectedly required Git: %v", err)
	}
	bare := filepath.Join(t.TempDir(), "source.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	v.Repositories[0].Source = bare
	if err := Validate(&v); err != nil {
		t.Fatalf("bare source should be rejected by Git preflight, not YAML validation: %v", err)
	}
}

func TestValidateRejectsDuplicateTargets(t *testing.T) {
	d := t.TempDir()
	v := task(d)
	v.Repositories = append(v.Repositories, domain.Repository{Name: "two", Source: d, Worktree: filepath.Join("worktrees", "one")})
	if err := Validate(&v); err == nil || !strings.Contains(err.Error(), "same worktree target") {
		t.Fatalf("expected duplicate target error, got %v", err)
	}
}

func TestValidateRejectsWorktreeEscape(t *testing.T) {
	d := t.TempDir()
	v := task(d)
	v.Repositories[0].Worktree = filepath.Join("..", "outside")
	if err := Validate(&v); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected containment error, got %v", err)
	}
}

func TestValidateNormalizesAndRejectsLocalOverlayPaths(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "local.env")
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	v := task(d)
	v.Repositories[0].Local = &domain.LocalOverlay{Paths: []string{"dir", `config\..\local.env`}}
	if err := Validate(&v); err != nil {
		t.Fatal(err)
	}
	if got := v.Repositories[0].Local.Paths[1]; got != "local.env" {
		t.Fatalf("normalized local path=%q", got)
	}

	for _, paths := range [][]string{
		{"../outside"},
		{filepath.Join(".git", "config")},
		{"local.env", filepath.Join("config", "..", "local.env")},
		{""},
	} {
		candidate := task(d)
		candidate.Repositories[0].Local = &domain.LocalOverlay{Paths: paths}
		if err := Validate(&candidate); err == nil {
			t.Fatalf("paths %#v unexpectedly passed", paths)
		}
	}
}

func TestLoadRejectsUnknownLocalOverlayField(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "taskflow.yaml")
	contents := "task:\n  id: A\nrepositories:\n  - name: one\n    source: " + d + "\n    local:\n      unexpected: true\n"
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("expected unknown local field rejection, got %v", err)
	}
}
