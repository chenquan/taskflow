package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenquan/specflow/internal/domain"
	"github.com/chenquan/specflow/internal/execx"
	"github.com/chenquan/specflow/internal/report"
)

func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", dir}, {"-C", dir, "config", "user.email", "test@example.com"}, {"-C", dir, "config", "user.name", "Test"}, {"-C", dir, "commit", "--allow-empty", "-m", "init"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

func TestLifecycleDoesNotRequireRequirementFile(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	options := InitOptions{TasksRoot: tasks, TaskID: "TASK-1", Primary: "repo", Repositories: []string{"repo=" + repo}}
	if result, code := s.Init(context.Background(), options); code != report.ExitOK || !result.OK {
		t.Fatalf("init: code=%d result=%#v", code, result)
	}
	raw, err := os.ReadFile(filepath.Join(tasks, "TASK-1", "specflow.yaml"))
	if err != nil || strings.Contains(string(raw), "openspec") {
		t.Fatalf("generated configuration contains OpenSpec: %s (%v)", raw, err)
	}
	task, err := s.Load(tasks, "TASK-1")
	if err != nil {
		t.Fatal(err)
	}
	if result, code := s.Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("start: code=%d result=%#v", code, result)
	}
	if result, code := s.Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("repeat start: code=%d result=%#v", code, result)
	}
	status, code := s.Status(context.Background(), task)
	if code != report.ExitOK || status.Data.(domain.StatusData).Repositories[0].Branch != "feature/task-1" {
		t.Fatalf("status: code=%d result=%#v", code, status)
	}
	if result, code := s.Validate(context.Background(), task); code != report.ExitOK || !result.OK {
		t.Fatalf("validate: code=%d result=%#v", code, result)
	}
}

func TestLoadStartStateCompatibility(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, ".specflow")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}}
	write := func(value any) []byte {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stateDir, "state.json"), raw, 0644); err != nil {
			t.Fatal(err)
		}
		return raw
	}

	legacy := write(domain.State{SchemaVersion: 1, TaskID: "TASK", Phase: "initialized"})
	state, exists, err := loadStartState(task)
	if err != nil || !exists || state.ConfigDigest != "" {
		t.Fatalf("legacy state: exists=%v err=%v state=%#v", exists, err, state)
	}
	corrupt := []byte("not-json")
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), corrupt, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadStartState(task); err == nil {
		t.Fatal("expected corrupt state error")
	}
	if got, err := os.ReadFile(filepath.Join(stateDir, "state.json")); err != nil || string(got) != string(corrupt) {
		t.Fatalf("corrupt state changed: %q %v", got, err)
	}
	wrong := write(domain.State{SchemaVersion: 2, TaskID: "TASK"})
	if _, _, err := loadStartState(task); err == nil {
		t.Fatal("expected incompatible schema error")
	}
	if got, err := os.ReadFile(filepath.Join(stateDir, "state.json")); err != nil || string(got) != string(wrong) {
		t.Fatalf("incompatible state changed: %q %v", got, err)
	}
	_ = legacy
}

func hasDiagnostic(diagnostics []report.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func TestPathAndRemoteHelpers(t *testing.T) {
	if !samePath(".", ".") || samePath(".", filepath.Join(".", "other")) {
		t.Fatal("unexpected path comparison")
	}
	if fetchRemote("upstream/main") != "upstream" || fetchRemote("") != "origin" {
		t.Fatal("unexpected remote derivation")
	}
}

func TestValidationReportRoundTripAndCompatibility(t *testing.T) {
	root := t.TempDir()
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}}
	reportValue := domain.ValidationReport{SchemaVersion: 1, TaskID: "TASK", Repositories: map[string]domain.RepositoryValidation{}}
	if err := persistValidationReport(task, reportValue); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadValidationReport(task)
	if err != nil || loaded.TaskID != "TASK" {
		t.Fatalf("loaded=%#v err=%v", loaded, err)
	}
	if err := os.WriteFile(validationReportPath(task), []byte("not-json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadValidationReport(task); err == nil {
		t.Fatal("expected invalid report error")
	}
	if err := persistValidationReport(task, domain.ValidationReport{SchemaVersion: 2, TaskID: "TASK", Repositories: map[string]domain.RepositoryValidation{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := loadValidationReport(task); err == nil {
		t.Fatal("expected incompatible report error")
	}
}

type openRunner struct {
	err error
}

func (r openRunner) Run(context.Context, execx.CommandSpec) (execx.Result, error) {
	if r.err != nil {
		return execx.Result{ExitCode: 7}, r.err
	}
	return execx.Result{}, nil
}

func (openRunner) LookPath(string) (string, error) { return "", nil }

func TestOpenReportsToolSuccessAndFailure(t *testing.T) {
	task := domain.Task{
		Task:         domain.TaskInfo{ID: "TASK", Root: t.TempDir()},
		Primary:      "repo",
		Repositories: []domain.Repository{{Name: "repo", Worktree: "worktrees/repo"}},
		Development:  domain.Development{DefaultTool: "codex", Tools: map[string]domain.ToolDef{"codex": {Executable: "codex"}}},
	}
	service := Service{Runner: openRunner{}}
	if result, code := service.Open(context.Background(), task, "", nil, nil, nil); code != report.ExitOK || !result.OK {
		t.Fatalf("success: code=%d result=%#v", code, result)
	}
	service.Runner = openRunner{err: os.ErrClosed}
	if result, code := service.Open(context.Background(), task, "codex", nil, nil, nil); code != report.ExitExecution || result.OK || !hasDiagnostic(result.Errors, "TOOL_EXITED") {
		t.Fatalf("failure: code=%d result=%#v", code, result)
	}
	if result, code := service.Open(context.Background(), task, "unknown", nil, nil, nil); code != report.ExitConfig || result.OK {
		t.Fatalf("invalid tool: code=%d result=%#v", code, result)
	}
}
