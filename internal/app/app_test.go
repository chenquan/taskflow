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
	"github.com/chenquan/taskflow/internal/ownership"
	"github.com/chenquan/taskflow/internal/plan"
	"github.com/chenquan/taskflow/internal/report"
)

func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main", dir}, {"-C", dir, "config", "user.email", "test@example.com"}, {"-C", dir, "config", "user.name", "Test"}, {"-C", dir, "commit", "--allow-empty", "-m", "init"}, {"-C", dir, "remote", "add", "origin", "https://example.test/repo.git"}, {"-C", dir, "update-ref", "refs/remotes/origin/main", "HEAD"}, {"-C", dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"}} {
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

func TestCreateRequiresRemoteDefaultBase(t *testing.T) {
	repo := makeGitRepo(t)
	if out, err := exec.Command("git", "-C", repo, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD").CombinedOutput(); err != nil {
		t.Fatalf("remove origin/HEAD: %v: %s", err, out)
	}
	result, code := New().Create(context.Background(), CreateOptions{
		TasksRoot:    t.TempDir(),
		TaskID:       "NO-REMOTE-DEFAULT",
		Repositories: []string{"repo=" + repo},
		DryRun:       true,
	})
	if code != report.ExitEnvironment || result.OK || !hasDiagnostic(result.Errors, "REMOTE_DEFAULT_UNAVAILABLE") {
		t.Fatalf("expected remote default failure: code=%d result=%#v", code, result)
	}
}

func TestCreateRejectsMalformedRepository(t *testing.T) {
	result, code := New().Create(context.Background(), CreateOptions{
		TasksRoot:    t.TempDir(),
		TaskID:       "BAD-REPO",
		Repositories: []string{"malformed"},
		DryRun:       true,
	})
	if code != report.ExitConfig || result.OK || !hasDiagnostic(result.Errors, "INVALID_REPOSITORY") {
		t.Fatalf("expected malformed repository failure: code=%d result=%#v", code, result)
	}
}

func TestCreateRejectsConflictingModes(t *testing.T) {
	result, code := New().Create(context.Background(), CreateOptions{
		TasksRoot: t.TempDir(),
		TaskID:    "CONFLICTING-MODES",
		DryRun:    true,
		Execute:   true,
	})
	if code != report.ExitConfig || result.OK || !hasDiagnostic(result.Errors, "INVALID_ARGUMENT") {
		t.Fatalf("expected conflicting mode failure: code=%d result=%#v", code, result)
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

func TestCreateRejectsRepositoryArgumentsForExistingTask(t *testing.T) {
	repo1, repo2 := makeGitRepo(t), makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if _, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "EXISTING", Repositories: []string{"one=" + repo1}, Execute: true}); code != report.ExitOK {
		t.Fatal(code)
	}
	configPath := filepath.Join(tasks, "EXISTING", "taskflow.yaml")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, options := range []CreateOptions{
		{TasksRoot: tasks, TaskID: "EXISTING", Repositories: []string{"two=" + repo2}, DryRun: true},
		{TasksRoot: tasks, TaskID: "EXISTING", Repositories: []string{"malformed"}, Execute: true},
	} {
		result, code := service.Create(context.Background(), options)
		if code != report.ExitConfig || result.OK || !hasDiagnostic(result.Errors, "CONFIG_EDIT_REQUIRED") {
			t.Fatalf("expected config edit diagnostic: code=%d result=%#v", code, result)
		}
	}
	after, _ := os.ReadFile(configPath)
	if string(before) != string(after) {
		t.Fatal("existing-task repository arguments changed taskflow.yaml")
	}
	if _, err := os.Stat(filepath.Join(tasks, "EXISTING", "worktrees", "two")); !os.IsNotExist(err) {
		t.Fatalf("existing-task repository arguments created worktree: %v", err)
	}
	if _, err := service.Git.Worktrees(context.Background(), repo2); err != nil {
		t.Fatal(err)
	}
	if task, err := service.Load(tasks, "EXISTING"); err != nil || len(task.Repositories) != 1 || task.Repositories[0].Name != "one" {
		t.Fatalf("existing task changed: %#v err=%v", task.Repositories, err)
	}
}

func TestCreateReconcilesDirectConfigurationEditsWithoutDeletingWorktrees(t *testing.T) {
	repo1, repo2 := makeGitRepo(t), makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if _, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "EDIT", Repositories: []string{"one=" + repo1}, Execute: true}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := service.Load(tasks, "EDIT")
	if err != nil {
		t.Fatal(err)
	}
	task.Repositories = append(task.Repositories, domain.Repository{
		Name:     "two",
		Source:   repo2,
		Base:     "origin/main",
		Branch:   "feature/edit",
		Worktree: filepath.Join("worktrees", "two"),
	})
	configPath := filepath.Join(task.Task.Root, "taskflow.yaml")
	raw, err := config.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "EDIT", Execute: true})
	if code != report.ExitOK || !result.OK {
		t.Fatalf("direct config reconcile: code=%d result=%#v", code, result)
	}
	after, _ := os.ReadFile(configPath)
	if string(before) != string(after) {
		t.Fatal("reconciliation rewrote user-owned taskflow.yaml")
	}
	for _, name := range []string{"one", "two"} {
		if _, err := os.Stat(filepath.Join(task.Task.Root, "worktrees", name)); err != nil {
			t.Fatalf("worktree %s missing: %v", name, err)
		}
	}

	task.Repositories = task.Repositories[:1]
	raw, err = config.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "EDIT", Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("direct config removal reconcile: code=%d result=%#v", code, result)
	}
	if _, err := os.Stat(filepath.Join(task.Task.Root, "worktrees", "two")); err != nil {
		t.Fatalf("unlisted worktree was removed: %v", err)
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

func TestDeleteRejectsInvalidArgumentsAndMissingTask(t *testing.T) {
	service := New()
	root := t.TempDir()
	cases := []struct {
		name string
		want string
		opt  DeleteOptions
	}{
		{name: "missing arguments", want: "INVALID_ARGUMENT", opt: DeleteOptions{TaskID: "TASK"}},
		{name: "conflicting modes", want: "INVALID_ARGUMENT", opt: DeleteOptions{TasksRoot: root, TaskID: "TASK", DryRun: true, Execute: true}},
		{name: "force without execute", want: "INVALID_ARGUMENT", opt: DeleteOptions{TasksRoot: root, TaskID: "TASK", Force: true}},
		{name: "invalid task id", want: "INVALID_TASK_ID", opt: DeleteOptions{TasksRoot: root, TaskID: "../TASK"}},
		{name: "missing tasks root", want: "TASKS_ROOT_NOT_FOUND", opt: DeleteOptions{TasksRoot: filepath.Join(root, "missing"), TaskID: "TASK"}},
		{name: "missing task", want: "TASK_NOT_FOUND", opt: DeleteOptions{TasksRoot: root, TaskID: "TASK"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, code := service.Delete(context.Background(), tc.opt)
			if result.OK || !hasDiagnostic(result.Errors, tc.want) || code == report.ExitOK {
				t.Fatalf("code=%d result=%#v", code, result)
			}
		})
	}
}

func TestDeleteRejectsInvalidOwnership(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if _, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "OWNERSHIP", Repositories: []string{"repo=" + repo}, Execute: true}); code != report.ExitOK {
		t.Fatal(code)
	}
	taskRoot := filepath.Join(tasks, "OWNERSHIP")
	ownershipPath := filepath.Join(taskRoot, ".taskflow", "ownership.json")
	if err := os.WriteFile(ownershipPath, []byte("not-json"), 0644); err != nil {
		t.Fatal(err)
	}
	if result, code := service.Delete(context.Background(), DeleteOptions{TasksRoot: tasks, TaskID: "OWNERSHIP"}); code != report.ExitConfig || result.OK || !hasDiagnostic(result.Errors, "INVALID_OWNERSHIP") {
		t.Fatalf("malformed ownership: code=%d result=%#v", code, result)
	}
	if err := os.WriteFile(ownershipPath, []byte(`{"version":1,"taskID":"OTHER","worktrees":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if result, code := service.Delete(context.Background(), DeleteOptions{TasksRoot: tasks, TaskID: "OWNERSHIP"}); code != report.ExitConfig || result.OK || !hasDiagnostic(result.Errors, "INVALID_OWNERSHIP") {
		t.Fatalf("ownership task mismatch: code=%d result=%#v", code, result)
	}
}

func TestDeleteRejectsConfigurationOwnershipMismatchAndProtectedBranch(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if _, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "MISMATCH", Repositories: []string{"repo=" + repo}, Execute: true}); code != report.ExitOK {
		t.Fatal(code)
	}
	task, err := service.Load(tasks, "MISMATCH")
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(task.Task.Root, "taskflow.yaml")
	task.Repositories[0].Branch = "feature/other"
	raw, err := config.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if result, code := service.Delete(context.Background(), DeleteOptions{TasksRoot: tasks, TaskID: "MISMATCH"}); code != report.ExitConflict || result.OK || !hasDiagnostic(result.Errors, "OWNERSHIP_MISMATCH") {
		t.Fatalf("configuration mismatch: code=%d result=%#v", code, result)
	}

	if err := os.WriteFile(configPath, mustMarshalTask(t, task, "main"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest, exists, err := ownership.Load(task.Task.Root)
	if err != nil || !exists {
		t.Fatalf("load ownership: exists=%v err=%v", exists, err)
	}
	manifest.Worktrees[0].Branch = "main"
	if err := ownership.Save(task.Task.Root, manifest); err != nil {
		t.Fatal(err)
	}
	if result, code := service.Delete(context.Background(), DeleteOptions{TasksRoot: tasks, TaskID: "MISMATCH"}); code != report.ExitConflict || result.OK || !hasDiagnostic(result.Errors, "PROTECTED_BRANCH") {
		t.Fatalf("protected branch: code=%d result=%#v", code, result)
	}
}

func TestDeleteHandlesAlreadyRemovedOwnedResources(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if _, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "ALREADY-REMOVED", Repositories: []string{"repo=" + repo}, Execute: true}); code != report.ExitOK {
		t.Fatal(code)
	}
	taskRoot := filepath.Join(tasks, "ALREADY-REMOVED")
	target := filepath.Join(taskRoot, "worktrees", "repo")
	if output, err := exec.Command("git", "-C", repo, "worktree", "remove", target).CombinedOutput(); err != nil {
		t.Fatalf("remove worktree: %v: %s", err, output)
	}
	if output, err := exec.Command("git", "-C", repo, "branch", "-D", "feature/already-removed").CombinedOutput(); err != nil {
		t.Fatalf("remove branch: %v: %s", err, output)
	}
	preview, code := service.Delete(context.Background(), DeleteOptions{TasksRoot: tasks, TaskID: "ALREADY-REMOVED"})
	if code != report.ExitOK || !preview.OK {
		t.Fatalf("preview: code=%d result=%#v", code, preview)
	}
	if result, code := service.Delete(context.Background(), DeleteOptions{TasksRoot: tasks, TaskID: "ALREADY-REMOVED", Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("execute: code=%d result=%#v", code, result)
	}
	if _, err := os.Stat(taskRoot); !os.IsNotExist(err) {
		t.Fatalf("task root remains: %v", err)
	}
}

func mustMarshalTask(t *testing.T, task domain.Task, branch string) []byte {
	t.Helper()
	task.Repositories[0].Branch = branch
	raw, err := config.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestGitErrorMessage(t *testing.T) {
	if got := gitErrorMessage("prefix", nil); got != "prefix" {
		t.Fatalf("nil error message=%q", got)
	}
	if got := gitErrorMessage("prefix", errors.New("boom")); got != "prefix: boom" {
		t.Fatalf("error message=%q", got)
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
	if runner.spec.Dir != filepath.Join(task.Task.Root, task.Repositories[0].Worktree) || runner.spec.Executable != filepath.Join("/tools", "codex") {
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
