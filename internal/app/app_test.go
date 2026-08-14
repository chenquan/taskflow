package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenquan/taskflow/internal/config"
	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/execx"
	"github.com/chenquan/taskflow/internal/git"
	"github.com/chenquan/taskflow/internal/plan"
	"github.com/chenquan/taskflow/internal/report"
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

func TestCreateDryRunDoesNotWrite(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "TASK-DRY", Repositories: []string{"repo=" + repo}, DryRun: true})
	if code != report.ExitOK || !result.OK {
		t.Fatalf("create dry-run: code=%d result=%#v", code, result)
	}
	if _, err := os.Stat(filepath.Join(tasks, "TASK-DRY")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created task directory: %v", err)
	}
	if _, err := service.Git.Worktrees(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	data := result.Data.(map[string]any)
	if data["dryRun"] != true {
		t.Fatalf("dry-run data: %#v", data)
	}
}

func TestCreateIsIdempotentAndDoesNotPersistState(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "TASK", Repositories: []string{"repo=" + repo}, Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("create: code=%d result=%#v", code, result)
	}
	task, err := service.Load(tasks, "TASK")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(task.Task.Root, task.Repositories[0].Worktree)
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("worktree missing: %v", err)
	}
	for _, legacy := range []string{"state.json", "inventory.json", filepath.Join("reports", "validation.json")} {
		if _, err := os.Stat(filepath.Join(task.Task.Root, ".taskflow", legacy)); !os.IsNotExist(err) {
			t.Fatalf("legacy artifact exists %s: %v", legacy, err)
		}
	}
	second, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "TASK", Execute: true})
	if code != report.ExitOK || !second.OK {
		t.Fatalf("repeat create: code=%d result=%#v", code, second)
	}
	items := second.Data.(map[string]any)["actions"].([]plan.Item)
	if len(items) != 1 || items[0].Status != "reuse" {
		t.Fatalf("repeat actions: %#v", items)
	}
}

func TestCreateAppendsRepositoryAndDryRunIsNonMutating(t *testing.T) {
	repo1, repo2 := makeGitRepo(t), makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if _, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "APPEND", Repositories: []string{"one=" + repo1}, Execute: true}); code != report.ExitOK {
		t.Fatal(code)
	}
	configPath := filepath.Join(tasks, "APPEND", "taskflow.yaml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	preview, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "APPEND", Repositories: []string{"two=" + repo2}, DryRun: true})
	if code != report.ExitOK || !preview.OK {
		t.Fatalf("append dry-run: code=%d result=%#v", code, preview)
	}
	after, _ := os.ReadFile(configPath)
	if string(before) != string(after) {
		t.Fatal("append dry-run changed taskflow.yaml")
	}
	if _, err := os.Stat(filepath.Join(tasks, "APPEND", "worktrees", "two")); !os.IsNotExist(err) {
		t.Fatalf("append dry-run created worktree: %v", err)
	}
	if result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "APPEND", Repositories: []string{"two=" + repo2}, Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("append execute: code=%d result=%#v", code, result)
	}
	task, err := service.Load(tasks, "APPEND")
	if err != nil || len(task.Repositories) != 2 || task.Repositories[1].Name != "two" {
		t.Fatalf("appended task: %#v err=%v", task.Repositories, err)
	}
	if _, err := os.Stat(filepath.Join(task.Task.Root, task.Repositories[1].Worktree)); err != nil {
		t.Fatalf("appended worktree missing: %v", err)
	}
}

func TestCreateRetriesAfterPartialGitFailureWithoutState(t *testing.T) {
	repo1, repo2 := makeGitRepo(t), makeGitRepo(t)
	tasks := t.TempDir()
	failing := &failSecondWorktreeRunner{}
	service := Service{Runner: failing, Git: git.Client{Runner: failing}}
	result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "RETRY", Repositories: []string{"one=" + repo1, "two=" + repo2}, Execute: true})
	if code != report.ExitPartial || result.OK || !hasDiagnostic(result.Errors, "CREATE_WORKTREE_FAILED") {
		t.Fatalf("expected partial create: code=%d result=%#v", code, result)
	}
	first := filepath.Join(tasks, "RETRY", "worktrees", "one")
	second := filepath.Join(tasks, "RETRY", "worktrees", "two")
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("first worktree was not retained: %v", err)
	}
	if _, err := os.Stat(second); !os.IsNotExist(err) {
		t.Fatalf("second worktree unexpectedly exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tasks, "RETRY", ".taskflow", "state.json")); !os.IsNotExist(err) {
		t.Fatalf("partial create wrote state: %v", err)
	}
	service.Runner = execx.OSRunner{}
	service.Git = git.Client{Runner: execx.OSRunner{}}
	if result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "RETRY", Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("retry: code=%d result=%#v", code, result)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("retry did not create second worktree: %v", err)
	}
}

func TestCreateRejectsMismatchedTargetBeforeMutation(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	taskRoot := filepath.Join(tasks, "CONFLICT")
	if err := os.MkdirAll(filepath.Join(taskRoot, "worktrees", "repo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskRoot, "worktrees", "repo", "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	task := domain.Task{Version: domain.ConfigVersion, Task: domain.TaskInfo{ID: "CONFLICT", Root: taskRoot}, Repositories: []domain.Repository{{Name: "repo", Source: repo, Base: "HEAD", Branch: "feature/conflict", Worktree: filepath.Join("worktrees", "repo")}}}
	raw, err := config.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskRoot, "taskflow.yaml"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(taskRoot, "worktrees", "repo", "keep.txt"))
	result, code := New().Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "CONFLICT", Execute: true})
	if code != report.ExitConflict || result.OK || !hasDiagnostic(result.Errors, "WORKTREE_MISMATCH") {
		t.Fatalf("expected target conflict: code=%d result=%#v", code, result)
	}
	after, _ := os.ReadFile(filepath.Join(taskRoot, "worktrees", "repo", "keep.txt"))
	if string(before) != string(after) {
		t.Fatal("mismatched target changed")
	}
}

func TestLoadRejectsLegacyRuntimeArtifacts(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if _, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "LEGACY", Repositories: []string{"repo=" + repo}, Execute: true}); code != report.ExitOK {
		t.Fatal(code)
	}
	legacy := filepath.Join(tasks, "LEGACY", ".taskflow", "state.json")
	if err := os.WriteFile(legacy, []byte(`{"phase":"started"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Load(tasks, "LEGACY"); err == nil || !strings.Contains(err.Error(), "legacy runtime artifact") {
		t.Fatalf("expected legacy artifact error, got %v", err)
	}
}

type openRunner struct {
	err         error
	lookPathErr error
	calls       int
	spec        execx.CommandSpec
}

type failSecondWorktreeRunner struct {
	adds int
}

func (r *failSecondWorktreeRunner) Run(ctx context.Context, spec execx.CommandSpec) (execx.Result, error) {
	if spec.Executable == "git" && containsArg(spec.Args, "worktree") && containsArg(spec.Args, "add") {
		r.adds++
		if r.adds == 2 {
			return execx.Result{ExitCode: 1, Stderr: "injected worktree failure"}, errors.New("injected worktree failure")
		}
	}
	return (execx.OSRunner{}).Run(ctx, spec)
}

func (r *failSecondWorktreeRunner) LookPath(name string) (string, error) {
	return (execx.OSRunner{}).LookPath(name)
}

func containsArg(args []string, expected string) bool {
	for _, arg := range args {
		if arg == expected {
			return true
		}
	}
	return false
}

func (r *openRunner) Run(_ context.Context, spec execx.CommandSpec) (execx.Result, error) {
	r.calls++
	r.spec = spec
	if r.err != nil {
		return execx.Result{ExitCode: 7}, r.err
	}
	return execx.Result{}, nil
}

func (r *openRunner) LookPath(name string) (string, error) {
	if r.lookPathErr != nil {
		return "", r.lookPathErr
	}
	return filepath.Join("/tools", name), nil
}

func TestOpenUsesLiveIdentityAndAllowsDirtyWorktree(t *testing.T) {
	repo1, repo2 := makeGitRepo(t), makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if _, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "OPEN", Repositories: []string{"one=" + repo1, "two=" + repo2}, Execute: true}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := service.Load(tasks, "OPEN")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(task.Task.Root, task.Repositories[0].Worktree, "dirty.txt"), []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}
	runner := &openRunner{}
	service.Runner = runner
	result, code := service.Open(context.Background(), task, "", []string{"--model", "test"}, nil, nil, nil)
	if code != report.ExitOK || !result.OK || runner.calls != 1 {
		t.Fatalf("open: code=%d calls=%d result=%#v", code, runner.calls, result)
	}
	if runner.spec.Dir != filepath.Join(task.Task.Root, task.Repositories[0].Worktree) || runner.spec.Executable != "/tools/codex" {
		t.Fatalf("launch spec: %#v", runner.spec)
	}
	if !containsPair(runner.spec.Args, "--model", "test") {
		t.Fatalf("extra args not forwarded: %#v", runner.spec.Args)
	}
	runner = &openRunner{err: errors.New("child failed")}
	service.Runner = runner
	if result, code := service.Open(context.Background(), task, "codex", nil, nil, nil, nil); code != report.ExitExecution || result.OK || !hasDiagnostic(result.Errors, "TOOL_EXITED") {
		t.Fatalf("child failure: code=%d result=%#v", code, result)
	}
}

func containsPair(values []string, first, second string) bool {
	for i := 0; i+1 < len(values); i++ {
		if values[i] == first && values[i+1] == second {
			return true
		}
	}
	return false
}

func hasDiagnostic(diagnostics []report.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
