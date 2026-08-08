package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chenquan/specflow/internal/domain"
	"github.com/chenquan/specflow/internal/execx"
	"github.com/chenquan/specflow/internal/git"
	"github.com/chenquan/specflow/internal/openspec"
	"github.com/chenquan/specflow/internal/plan"
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

type recordingRunner struct{ specs []execx.CommandSpec }

func (r *recordingRunner) Run(_ context.Context, spec execx.CommandSpec) (execx.Result, error) {
	r.specs = append(r.specs, spec)
	return execx.Result{}, nil
}
func (r *recordingRunner) LookPath(string) (string, error) { return "tool", nil }

func TestInitIsIdempotentAndDoesNotChangeRepository(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	o := InitOptions{TasksRoot: tasks, TaskID: "TASK-1", Primary: "repo", Repositories: []string{"repo=" + repo}}
	r, code := s.Init(context.Background(), o)
	if code != report.ExitOK || !r.OK {
		t.Fatalf("init: %#v", r)
	}
	if _, err := os.Stat(filepath.Join(tasks, "TASK-1", "specflow.yaml")); err != nil {
		t.Fatal(err)
	}
	r, code = s.Init(context.Background(), o)
	if code != report.ExitOK || r.Data.(map[string]any)["initialized"] != false {
		t.Fatalf("repeat init: %#v", r)
	}
	if out, err := exec.Command("git", "-C", repo, "status", "--porcelain").Output(); err != nil || len(out) != 0 {
		t.Fatalf("repository changed: %s %v", out, err)
	}
}

func TestInitRejectsTraversalTaskID(t *testing.T) {
	root := t.TempDir()
	repo := makeGitRepo(t)
	s := New()
	r, code := s.Init(context.Background(), InitOptions{TasksRoot: root, TaskID: "../outside", Primary: "repo", Repositories: []string{"repo=" + repo}})
	if code != report.ExitConfig || r.OK {
		t.Fatalf("%d %#v", code, r)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "outside")); !os.IsNotExist(err) {
		t.Fatalf("traversal wrote outside root: %v", err)
	}
}
func TestLoadRejectsTraversalTaskID(t *testing.T) {
	_, err := New().Load(t.TempDir(), "../outside")
	if err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestDoctorReportsDirtyRepositoryAndMissingOpenSpec(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks := t.TempDir()
	s := New()
	_, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-2", Primary: "repo", Repositories: []string{"repo=" + repo}})
	if code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := s.Load(tasks, "TASK-2")
	if err != nil {
		t.Fatal(err)
	}
	result, code := s.Doctor(context.Background(), task, "repo")
	if code != report.ExitEnvironment {
		t.Fatalf("code = %d, result = %#v", code, result)
	}
	seenDirty, seenOpenSpec := false, false
	for _, diagnostic := range append(result.Warnings, result.Errors...) {
		seenDirty = seenDirty || diagnostic.Code == "SOURCE_DIRTY"
		seenOpenSpec = seenOpenSpec || diagnostic.Code == "OPENSPEC_NOT_INITIALIZED"
	}
	if !seenDirty || !seenOpenSpec {
		t.Fatalf("missing expected diagnostics: %#v", result)
	}
}

func TestStartDryRunDoesNotCreateWorktree(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	_, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-3", Primary: "repo", Repositories: []string{"repo=" + repo}})
	if code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := s.Load(tasks, "TASK-3")
	if err != nil {
		t.Fatal(err)
	}
	result, code := s.Start(context.Background(), task, StartOptions{DryRun: true})
	if code != report.ExitOK || !result.OK {
		t.Fatalf("start dry-run: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(tasks, "TASK-3", "worktrees", "repo")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created target: %v", err)
	}
	if actions, ok := result.Data.(map[string]any)["actions"]; !ok || actions == nil {
		t.Fatalf("missing action plan: %#v", result.Data)
	}
}

func TestStartExecuteCreatesAndReusesWorktreeAndChange(t *testing.T) {
	if _, err := exec.LookPath("openspec"); err != nil {
		t.Skip("openspec CLI is unavailable")
	}
	repo := makeGitRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, "openspec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "openspec", "config.yaml"), []byte("schema: spec-driven\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks := t.TempDir()
	s := New()
	o := InitOptions{TasksRoot: tasks, TaskID: "TASK-4", Primary: "repo", Repositories: []string{"repo=" + repo}}
	if _, code := s.Init(context.Background(), o); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := s.Load(tasks, "TASK-4")
	if err != nil {
		t.Fatal(err)
	}
	if result, code := s.Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("execute: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(tasks, "TASK-4", "worktrees", "repo", "openspec", "changes", "task-4-repo")); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(tasks, "TASK-4", "worktrees", "repo")
	if err := os.WriteFile(filepath.Join(worktree, "dirty.txt"), []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}
	status, code := s.Status(context.Background(), task)
	if code != report.ExitOK {
		t.Fatal(code)
	}
	entries := status.Data.(map[string]any)["repositories"].([]any)
	entry := entries[0].(map[string]any)
	if entry["dirty"] != true || entry["branch"] != "feature/task-4" {
		t.Fatalf("unexpected status %#v", entry)
	}
	finish, code := s.Finish(context.Background(), task)
	if code != report.ExitValidation || finish.OK {
		t.Fatalf("dirty finish: %d %#v", code, finish)
	}
	task, err = s.Load(tasks, "TASK-4")
	if err != nil {
		t.Fatal(err)
	}
	if result, code := s.Start(context.Background(), task, StartOptions{Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("repeat execute: %#v", result)
	}
}
func TestStatusReportsActiveSession(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specflow"), 0755); err != nil {
		t.Fatal(err)
	}
	h, e := session.Acquire(root, "codex", "/tmp/primary")
	if e != nil {
		t.Fatal(e)
	}
	defer h.Release()
	s := New()
	task := domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}}
	r, _ := s.Status(context.Background(), task)
	data := r.Data.(map[string]any)
	lease, ok := data["activeSession"].(*session.Lease)
	if !ok || lease.Tool != "codex" {
		t.Fatalf("%#v", data)
	}
}
func TestValidateOrdersRepositories(t *testing.T) {
	root := t.TempDir()
	repos := []domain.Repository{{Name: "dependent", Worktree: "worktrees/dependent", Change: "dependent", DependsOn: []string{"owner"}, Checks: []domain.Check{{Name: "dependent", Executable: "dependent"}}}, {Name: "owner", Worktree: "worktrees/owner", Change: "owner", Checks: []domain.Check{{Name: "owner", Executable: "owner"}}}}
	for _, repo := range repos {
		if err := os.MkdirAll(filepath.Join(root, repo.Worktree, "openspec", "changes", repo.Change), 0755); err != nil {
			t.Fatal(err)
		}
	}
	runner := &recordingRunner{}
	s := Service{Runner: runner, Git: git.Client{Runner: runner}, OpenSpec: openspec.Client{Runner: runner}}
	_, code := s.Validate(context.Background(), domain.Task{Task: domain.TaskInfo{ID: "TASK", Root: root}, Repositories: repos})
	if code != report.ExitOK || len(runner.specs) != 2 || runner.specs[0].Executable != "owner" {
		t.Fatalf("%d %#v", code, runner.specs)
	}
}
func TestStartExecuteHonorsFetch(t *testing.T) {
	if _, err := exec.LookPath("openspec"); err != nil {
		t.Skip("openspec CLI is unavailable")
	}
	repo := makeGitRepo(t)
	remote := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare remote: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "remote", "add", "origin", remote).CombinedOutput(); err != nil {
		t.Fatalf("remote add: %v: %s", err, out)
	}
	if err := os.MkdirAll(filepath.Join(repo, "openspec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "openspec", "config.yaml"), []byte("schema: spec-driven\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks := t.TempDir()
	s := New()
	if _, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-6", Primary: "repo", Repositories: []string{"repo=" + repo}}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, e := s.Load(tasks, "TASK-6")
	if e != nil {
		t.Fatal(e)
	}
	task.Execution.Fetch = true
	r, code := s.Start(context.Background(), task, StartOptions{Execute: true})
	if code != report.ExitOK || !r.OK {
		t.Fatalf("fetch start: %#v", r)
	}
	items := r.Data.(map[string]any)["actions"].([]plan.Item)
	if items[0].Kind != "fetch" {
		t.Fatalf("missing fetch action: %#v", items)
	}
}
func TestFinishReturnsValidationFailureForMissingChange(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	s := New()
	if _, code := s.Init(context.Background(), InitOptions{TasksRoot: tasks, TaskID: "TASK-5", Primary: "repo", Repositories: []string{"repo=" + repo}}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, e := s.Load(tasks, "TASK-5")
	if e != nil {
		t.Fatal(e)
	}
	r, code := s.Finish(context.Background(), task)
	if code != report.ExitValidation || r.OK {
		t.Fatalf("%d %#v", code, r)
	}
}
