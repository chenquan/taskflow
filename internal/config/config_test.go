package config

import (
	"github.com/chenquan/taskflow/internal/domain"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func task(root string) domain.Task {
	return domain.Task{Version: domain.ConfigVersion, Task: domain.TaskInfo{ID: "A", Root: root}, Repositories: []domain.Repository{{Name: "one", Source: root}}}
}

func TestLoadRejectsRemovedAndUnknownFields(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", d).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	v := task(d)
	raw, err := Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "taskflow.yaml")
	removed := string(raw) + "\ncreate_openspec_change: true\n"
	if err := os.WriteFile(path, []byte(removed), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "create_openspec_change") {
		t.Fatalf("expected removed field rejection, got %v", err)
	}
	stale := string(raw) + "\nunknown_field: true\n"
	if err := os.WriteFile(path, []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "unknown_field") {
		t.Fatalf("expected unknown field rejection, got %v", err)
	}
}

func TestLoadRejectsRemovedDevelopmentConfiguration(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "taskflow.yaml")
	raw := "task:\n  id: A\nrepositories:\n  - name: one\n    source: " + d + "\ndevelopment:\n  default_tool: codex\n"
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "development") {
		t.Fatalf("expected removed configuration rejection, got %v", err)
	}
}
func TestValidateDefaults(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", d).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	v := task(d)
	if err := Validate(&v); err != nil {
		t.Fatal(err)
	}
	if v.Repositories[0].Worktree != filepath.Join("worktrees", "one") {
		t.Fatal(v.Repositories[0].Worktree)
	}
}

func TestValidateDerivesVersion(t *testing.T) {
	d := t.TempDir()
	v := task(d)
	v.Version = 0
	if err := Validate(&v); err != nil {
		t.Fatal(err)
	}
	if v.Version != domain.ConfigVersion {
		t.Fatalf("version=%d", v.Version)
	}
}
func TestValidateCycle(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", d).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	v := task(d)
	v.Repositories = append(v.Repositories, domain.Repository{Name: "two", Source: d, DependsOn: []string{"one"}})
	v.Repositories[0].DependsOn = []string{"two"}
	if Validate(&v) == nil {
		t.Fatal("expected cycle")
	}
}
func TestValidateAllowsExistingNonGitSourceStructurally(t *testing.T) {
	d := t.TempDir()
	v := task(d)
	if err := Validate(&v); err != nil {
		t.Fatalf("structural validation unexpectedly inspected Git: %v", err)
	}
}
func TestValidateAllowsBareGitSourceStructurally(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(t.TempDir(), "source.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	v := task(root)
	v.Repositories[0].Source = bare
	if err := Validate(&v); err != nil {
		t.Fatalf("structural validation unexpectedly inspected Git: %v", err)
	}
}
func TestValidateRejectsInvalidCheck(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", d).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	v := task(d)
	v.Repositories[0].Checks = []domain.Check{{Name: "check", Executable: "echo", Timeout: "soon"}}
	if Validate(&v) == nil {
		t.Fatal("expected invalid timeout")
	}
}
