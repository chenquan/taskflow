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
	return domain.Task{Version: 1, Task: domain.TaskInfo{ID: "A", Root: root}, Primary: "one", Repositories: []domain.Repository{{Name: "one", Source: root}}, Development: domain.Development{DefaultTool: "codex", EnabledTools: []string{"codex"}, Tools: map[string]domain.ToolDef{"codex": {Executable: "codex", LaunchMode: "direct"}}}, Execution: domain.Execution{CreateOpenSpecChange: true}}
}

func TestLoadRequiresExplicitSupportedExecutionPolicy(t *testing.T) {
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
	missing := strings.Replace(string(raw), "    create_openspec_change: true\n", "", 1)
	if err := os.WriteFile(path, []byte(missing), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "create_openspec_change is required") {
		t.Fatalf("expected required execution intent, got %v", err)
	}
	stale := strings.Replace(string(raw), "    create_openspec_change: true\n", "    create_openspec_change: true\n    cleanup: false\n", 1)
	if err := os.WriteFile(path, []byte(stale), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "cleanup") {
		t.Fatalf("expected removed field rejection, got %v", err)
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
func TestValidateRejectsNonGitSource(t *testing.T) {
	d := t.TempDir()
	v := task(d)
	if Validate(&v) == nil {
		t.Fatal("expected non-git source rejection")
	}
}
func TestValidateRejectsBareGitSource(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(t.TempDir(), "source.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	v := task(root)
	v.Repositories[0].Source = bare
	if Validate(&v) == nil {
		t.Fatal("expected bare Git source rejection")
	}
}
func TestValidateRejectsInvalidCheckAndChange(t *testing.T) {
	d := t.TempDir()
	if out, err := exec.Command("git", "init", d).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	v := task(d)
	v.Repositories[0].Change = "My_Change"
	if Validate(&v) == nil {
		t.Fatal("expected invalid change")
	}
	v = task(d)
	v.Repositories[0].Checks = []domain.Check{{Name: "check", Executable: "echo", Timeout: "soon"}}
	if Validate(&v) == nil {
		t.Fatal("expected invalid timeout")
	}
}
