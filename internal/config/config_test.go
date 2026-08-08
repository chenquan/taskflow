package config

import (
	"github.com/chenquan/specflow/internal/domain"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func task(root string) domain.Task {
	return domain.Task{Version: 1, Task: domain.TaskInfo{ID: "A", Root: root}, Primary: "one", Repositories: []domain.Repository{{Name: "one", Source: root}}, Development: domain.Development{DefaultTool: "codex", EnabledTools: []string{"codex"}, Tools: map[string]domain.ToolDef{"codex": {Executable: "codex", LaunchMode: "direct"}}}}
}

func TestLoadAcceptsLegacyOpenSpecFieldAndRejectsOtherUnknownFields(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", d).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	v := task(d)
	raw, err := Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "specflow.yaml")
	legacy := strings.Replace(string(raw), "execution: {}", "execution:\n    create_openspec_change: true", 1)
	if err := os.WriteFile(path, []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("expected legacy configuration compatibility, got %v", err)
	}
	normalized, err := Marshal(loaded)
	if err != nil || strings.Contains(string(normalized), "create_openspec_change") {
		t.Fatalf("legacy field survived normalized output: %s (%v)", normalized, err)
	}
	stale := legacy + "\nunknown_field: true\n"
	if err := os.WriteFile(path, []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "unknown_field") {
		t.Fatalf("expected unknown field rejection, got %v", err)
	}
}

func TestValidateDevelopmentPolicy(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", d).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	v := task(d)
	v.Development.DefaultTool = "claude"
	if err := Validate(&v); err == nil || !strings.Contains(err.Error(), "not enabled") {
		t.Fatalf("expected disabled default rejection, got %v", err)
	}
	v = task(d)
	definition := v.Development.Tools["codex"]
	definition.LaunchMode = "shell"
	v.Development.Tools["codex"] = definition
	if err := Validate(&v); err == nil || !strings.Contains(err.Error(), "launch mode") {
		t.Fatalf("expected launch mode rejection, got %v", err)
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
