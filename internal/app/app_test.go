package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenquan/specflow/internal/domain"
	"github.com/chenquan/specflow/internal/report"
	"github.com/chenquan/specflow/internal/session"
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

func TestLifecycleDoesNotRequireOpenSpec(t *testing.T) {
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
	if result, code := s.Doctor(context.Background(), task, "repo"); code != report.ExitOK || !result.OK {
		t.Fatalf("doctor: code=%d result=%#v", code, result)
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
	if result, code := s.Finish(context.Background(), task); code != report.ExitOK || !result.OK {
		t.Fatalf("finish: code=%d result=%#v", code, result)
	}
}

func TestDoctorReportsDirtySourceWithoutOpenSpecError(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks := t.TempDir()
	s := New()
	if _, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-2", Primary: "repo", Repositories: []string{"repo=" + repo}}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := s.Load(tasks, "TASK-2")
	if err != nil {
		t.Fatal(err)
	}
	result, code := s.Doctor(context.Background(), task, "repo")
	if code != report.ExitOK || !hasDiagnostic(result.Warnings, "SOURCE_DIRTY") || hasDiagnostic(result.Errors, "OPENSPEC_NOT_INITIALIZED") {
		t.Fatalf("doctor: code=%d result=%#v", code, result)
	}
}

func TestStatusReportsActiveSession(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specflow"), 0755); err != nil {
		t.Fatal(err)
	}
	holder, err := session.Acquire(root, "codex", "/tmp/primary")
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	result, code := New().Status(context.Background(), domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}})
	data := result.Data.(domain.StatusData)
	lease, ok := data.ActiveSession.(*session.Lease)
	if code != report.ExitOK || !ok || lease.Tool != "codex" {
		t.Fatalf("status: code=%d result=%#v", code, result)
	}
}

func hasDiagnostic(diagnostics []report.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
