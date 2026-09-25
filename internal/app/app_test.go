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

func writeCopyManifest(t *testing.T, repo string, lines ...string) {
	t.Helper()
	raw := ""
	for _, line := range lines {
		raw += line + "\n"
	}
	if err := os.WriteFile(filepath.Join(repo, ".worktreeinclude"), []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCreateDryRunDoesNotWrite(t *testing.T) {
	repo := makeGitRepo(t)
	writeCopyManifest(t, repo, "# no local content to carry")
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
	writeCopyManifest(t, repo, "# no local content to carry")
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
	if len(items) != 2 || items[0].Kind != "worktree" || items[0].Status != "reuse" || items[1].Kind != "source-copy" || items[1].Status != "reuse" {
		t.Fatalf("repeat actions: %#v", items)
	}
}

func TestCreateRejectsRepositoryArgumentsForExistingTask(t *testing.T) {
	repo1, repo2 := makeGitRepo(t), makeGitRepo(t)
	writeCopyManifest(t, repo1, "# no local content to carry")
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
	writeCopyManifest(t, repo1, "# no local content to carry")
	writeCopyManifest(t, repo2, "# no local content to carry")
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
	writeCopyManifest(t, repo1, "# no local content to carry")
	writeCopyManifest(t, repo2, "# no local content to carry")
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

func TestCreateCopiesWhitelistedSourceContent(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("ignored.log\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-C", repo, "add", ".gitignore"}, {"-C", repo, "commit", "-m", "ignore"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("committed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-C", repo, "add", "tracked.txt"}, {"-C", repo, "commit", "-m", "tracked"}, {"-C", repo, "update-ref", "refs/remotes/origin/main", "HEAD"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("committed\nmodified\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("untracked"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "ignored.log"), []byte("ignored"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "extra.txt"), []byte("unlisted"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "config", "dev"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "config", "dev", "settings local.env"), []byte("debug=true"), 0640); err != nil {
		t.Fatal(err)
	}
	writeCopyManifest(t, repo, "tracked.txt", "untracked.txt", "ignored.log", "config/")

	tasks := t.TempDir()
	service := New()
	preview, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "WHITELIST", Repositories: []string{"app=" + repo}, DryRun: true})
	if code != report.ExitOK || !preview.OK {
		t.Fatalf("preview: code=%d result=%#v", code, preview)
	}
	previewItems := preview.Data.(map[string]any)["actions"].([]plan.Item)
	if len(previewItems) != 2 || previewItems[0].Status != "create" || previewItems[1].Kind != "source-copy" || previewItems[1].Status != "copy" || previewItems[1].PatternCount != 4 {
		t.Fatalf("preview actions: %#v", previewItems)
	}
	if _, err := os.Stat(filepath.Join(tasks, "WHITELIST")); !os.IsNotExist(err) {
		t.Fatalf("dry-run changed task root: %v", err)
	}

	result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "WHITELIST", Repositories: []string{"app=" + repo}, Execute: true})
	if code != report.ExitOK || !result.OK {
		t.Fatalf("execute: code=%d result=%#v", code, result)
	}
	executeItems := result.Data.(map[string]any)["actions"].([]plan.Item)
	if len(executeItems) != 2 || executeItems[0].Status != "created" || executeItems[1].Status != "copied" {
		t.Fatalf("execute actions: %#v", executeItems)
	}
	task, err := service.Load(tasks, "WHITELIST")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(task.Task.Root, task.Repositories[0].Worktree)
	for _, file := range []string{"tracked.txt", "untracked.txt", "ignored.log", filepath.Join("config", "dev", "settings local.env")} {
		if _, err := os.Stat(filepath.Join(target, file)); err != nil {
			t.Fatalf("whitelisted file %s missing: %v", file, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "extra.txt")); !os.IsNotExist(err) {
		t.Fatalf("unlisted file was copied: %v", err)
	}
	if contents, err := os.ReadFile(filepath.Join(target, "tracked.txt")); err != nil || string(contents) != "committed\nmodified\n" {
		t.Fatalf("tracked modification not copied: %q err=%v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("target lost its own git metadata: %v", err)
	}
	manifest, exists, err := ownership.Load(task.Task.Root)
	if err != nil || !exists || len(manifest.Worktrees) != 1 || manifest.Worktrees[0].SourceCopy == nil || manifest.Worktrees[0].SourceCopy.Status != "complete" {
		t.Fatalf("source-copy ownership: manifest=%#v exists=%v err=%v", manifest, exists, err)
	}

	status, err := exec.Command("git", "-C", target, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("target status: %v: %s", err, status)
	}
	statusText := string(status)
	if !strings.Contains(statusText, " M tracked.txt") || !strings.Contains(statusText, "?? untracked.txt") {
		t.Fatalf("copied changes are not normal working-tree changes: %q", statusText)
	}
	if strings.Contains(statusText, "D ") || strings.Contains(statusText, "D\t") {
		t.Fatalf("base checkout lost tracked files: %q", statusText)
	}

	// A completed source copy is a creation-time snapshot: later source
	// changes must not be refreshed into a reused worktree.
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("changed later\n"), 0644); err != nil {
		t.Fatal(err)
	}
	second, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "WHITELIST", Execute: true})
	if code != report.ExitOK || !second.OK {
		t.Fatalf("repeat create: code=%d result=%#v", code, second)
	}
	secondItems := second.Data.(map[string]any)["actions"].([]plan.Item)
	if len(secondItems) != 2 || secondItems[0].Status != "reuse" || secondItems[1].Status != "reuse" {
		t.Fatalf("repeat actions: %#v", secondItems)
	}
	if contents, err := os.ReadFile(filepath.Join(target, "tracked.txt")); err != nil || string(contents) != "committed\nmodified\n" {
		t.Fatalf("completed copy was refreshed: %q err=%v", contents, err)
	}
}

func TestCreateCarriesNothingForCommentOnlyManifest(t *testing.T) {
	repo := makeGitRepo(t)
	writeCopyManifest(t, repo, "# nothing to carry", "")
	tasks := t.TempDir()
	result, code := New().Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "EMPTY", Repositories: []string{"app=" + repo}, Execute: true})
	if code != report.ExitOK || !result.OK {
		t.Fatalf("create: code=%d result=%#v", code, result)
	}
	items := result.Data.(map[string]any)["actions"].([]plan.Item)
	if len(items) != 2 || items[0].Status != "created" || items[1].Status != "copied" || items[1].FileCount != 0 || items[1].TotalBytes != 0 {
		t.Fatalf("comment-only manifest actions: %#v", items)
	}
	status, err := exec.Command("git", "-C", filepath.Join(tasks, "EMPTY", "worktrees", "app"), "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("target status: %v: %s", err, status)
	}
	if len(strings.TrimSpace(string(status))) != 0 {
		t.Fatalf("comment-only manifest produced a dirty worktree: %q", status)
	}
}

func TestCreateExcludesNestedGitMetadata(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("base"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-C", repo, "add", "tracked.txt"}, {"-C", repo, "commit", "-m", "tracked"}, {"-C", repo, "update-ref", "refs/remotes/origin/main", "HEAD"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if output, err := exec.Command("git", "-C", repo, "worktree", "add", "./wt-nested", "-b", "nested").CombinedOutput(); err != nil {
		t.Fatalf("nested worktree: %v: %s", err, output)
	}
	if err := os.MkdirAll(filepath.Join(repo, "sub", ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "sub", ".git", "HEAD"), []byte("embedded"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "sub", "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	writeCopyManifest(t, repo, "wt-nested/", "sub/")
	tasks := t.TempDir()
	if result, code := New().Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "NESTED", Repositories: []string{"app=" + repo}, Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("create: code=%d result=%#v", code, result)
	}
	target := filepath.Join(tasks, "NESTED", "worktrees", "app")
	if contents, err := os.ReadFile(filepath.Join(target, "tracked.txt")); err != nil || string(contents) != "base" {
		t.Fatalf("unlisted tracked file lost its base checkout: %q err=%v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(target, "wt-nested", "tracked.txt")); err != nil {
		t.Fatalf("listed nested working files were not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "wt-nested", ".git")); !os.IsNotExist(err) {
		t.Fatalf("nested worktree registration was copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "sub", ".git")); !os.IsNotExist(err) {
		t.Fatalf("embedded repository metadata was copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "sub", "keep.txt")); err != nil {
		t.Fatalf("listed sibling working file missing: %v", err)
	}
}

func TestCreateDoesNotCopyIntoMatchingManualWorktree(t *testing.T) {
	repo := makeGitRepo(t)
	writeCopyManifest(t, repo, "local.env")
	if err := os.WriteFile(filepath.Join(repo, "local.env"), []byte("local"), 0600); err != nil {
		t.Fatal(err)
	}
	tasks := t.TempDir()
	taskRoot := filepath.Join(tasks, "MANUAL")
	target := filepath.Join(taskRoot, "worktrees", "app")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	client := git.Client{Runner: execx.OSRunner{}}
	if err := client.AddWorktree(context.Background(), repo, "feature/manual", target, "HEAD", false); err != nil {
		t.Fatal(err)
	}
	task := domain.Task{
		Version: domain.ConfigVersion,
		Task:    domain.TaskInfo{ID: "MANUAL", Root: taskRoot},
		Repositories: []domain.Repository{{
			Name:     "app",
			Source:   repo,
			Base:     "HEAD",
			Branch:   "feature/manual",
			Worktree: filepath.Join("worktrees", "app"),
		}},
	}
	raw, err := config.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskRoot, "taskflow.yaml"), raw, 0644); err != nil {
		t.Fatal(err)
	}
	result, code := New().Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "MANUAL", Execute: true})
	if code != report.ExitOK || !result.OK {
		t.Fatalf("manual reuse: code=%d result=%#v", code, result)
	}
	items := result.Data.(map[string]any)["actions"].([]plan.Item)
	if len(items) != 2 || items[0].Status != "reuse" || items[1].Status != "reuse" || items[1].Reason == "" {
		t.Fatalf("manual reuse actions: %#v", items)
	}
	if _, err := os.Stat(filepath.Join(target, "local.env")); !os.IsNotExist(err) {
		t.Fatalf("manual worktree received a source copy: %v", err)
	}
}

func TestCreateRepairsPendingSourceCopyWithoutRecreatingWorktree(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "one.env"), []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "two.env"), []byte("two"), 0600); err != nil {
		t.Fatal(err)
	}
	writeCopyManifest(t, repo, "one.env", "two.env")
	tasks := t.TempDir()
	service := New()
	if result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "PENDING", Repositories: []string{"app=" + repo}, Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("initial create: code=%d result=%#v", code, result)
	}
	task, err := service.Load(tasks, "PENDING")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(task.Task.Root, task.Repositories[0].Worktree)
	manifest, exists, err := ownership.Load(task.Task.Root)
	if err != nil || !exists {
		t.Fatalf("load manifest: exists=%v err=%v", exists, err)
	}
	manifest.Worktrees[0].SourceCopy.Status = "pending"
	if err := ownership.Save(task.Task.Root, manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(target, "two.env")); err != nil {
		t.Fatal(err)
	}
	result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "PENDING", Execute: true})
	if code != report.ExitOK || !result.OK {
		t.Fatalf("pending retry: code=%d result=%#v", code, result)
	}
	items := result.Data.(map[string]any)["actions"].([]plan.Item)
	if len(items) != 2 || items[0].Status != "reuse" || items[1].Status != "repaired" {
		t.Fatalf("pending retry actions: %#v", items)
	}
	if _, err := os.Stat(filepath.Join(target, "two.env")); err != nil {
		t.Fatalf("pending source copy was not repaired: %v", err)
	}
	manifest, _, err = ownership.Load(task.Task.Root)
	if err != nil || manifest.Worktrees[0].SourceCopy.Status != "complete" {
		t.Fatalf("pending source-copy status: %#v err=%v", manifest, err)
	}
}

func TestCreateRegistersMissingPendingTargetBeforeCopying(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "notes.txt"), []byte("notes"), 0644); err != nil {
		t.Fatal(err)
	}
	writeCopyManifest(t, repo, "notes.txt")
	tasks := t.TempDir()
	service := New()
	if result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "MISSING", Repositories: []string{"app=" + repo}, Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("initial create: code=%d result=%#v", code, result)
	}
	task, err := service.Load(tasks, "MISSING")
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(task.Task.Root, task.Repositories[0].Worktree)
	manifest, exists, err := ownership.Load(task.Task.Root)
	if err != nil || !exists {
		t.Fatalf("load manifest: exists=%v err=%v", exists, err)
	}
	manifest.Worktrees[0].SourceCopy.Status = "pending"
	if err := ownership.Save(task.Task.Root, manifest); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", repo, "worktree", "remove", "--force", target).CombinedOutput(); err != nil {
		t.Fatalf("remove worktree: %v: %s", err, output)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still present: %v", err)
	}
	result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "MISSING", Execute: true})
	if code != report.ExitOK || !result.OK {
		t.Fatalf("missing pending retry: code=%d result=%#v", code, result)
	}
	items := result.Data.(map[string]any)["actions"].([]plan.Item)
	if len(items) != 2 || items[0].Status != "created" || items[1].Status != "copied" {
		t.Fatalf("missing pending actions: %#v", items)
	}
	if _, err := os.Stat(filepath.Join(target, "notes.txt")); err != nil {
		t.Fatalf("re-registered target was not copied: %v", err)
	}
	manifest, _, err = ownership.Load(task.Task.Root)
	if err != nil || manifest.Worktrees[0].SourceCopy.Status != "complete" {
		t.Fatalf("source-copy status after re-registration: %#v err=%v", manifest, err)
	}
}

func TestCreateRejectsSourceTargetOverlapBeforeMutation(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := filepath.Join(repo, ".tasks")
	if err := os.MkdirAll(tasks, 0755); err != nil {
		t.Fatal(err)
	}
	result, code := New().Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "OVERLAP", Repositories: []string{"app=" + repo}, DryRun: true})
	if code != report.ExitConflict || result.OK || !hasDiagnostic(result.Errors, "SOURCE_COPY_BOUNDARY") {
		t.Fatalf("expected source/target boundary conflict: code=%d result=%#v", code, result)
	}
	if _, err := os.Stat(filepath.Join(tasks, "OVERLAP")); !os.IsNotExist(err) {
		t.Fatalf("boundary conflict changed task root: %v", err)
	}
}

func TestLoadRejectsLegacyRuntimeArtifacts(t *testing.T) {
	repo := makeGitRepo(t)
	writeCopyManifest(t, repo, "# no local content to carry")
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
	writeCopyManifest(t, repo, "# no local content to carry")
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
	writeCopyManifest(t, repo, "# no local content to carry")
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
	writeCopyManifest(t, repo, "# no local content to carry")
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

func TestCreateFailsLoudlyWithoutManifest(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	preview, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "NOMANIFEST", Repositories: []string{"app=" + repo}, DryRun: true})
	if code != report.ExitEnvironment || preview.OK || !hasDiagnostic(preview.Errors, "SOURCE_COPY_MANIFEST_MISSING") {
		t.Fatalf("dry-run without manifest: code=%d result=%#v", code, preview)
	}
	if diagnosticHint(preview.Errors, "SOURCE_COPY_MANIFEST_MISSING") == "" {
		t.Fatalf("missing-manifest diagnostic has no hint: %#v", preview.Errors)
	}
	if _, err := os.Stat(filepath.Join(tasks, "NOMANIFEST")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created task directory: %v", err)
	}
	result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "NOMANIFEST", Repositories: []string{"app=" + repo}, Execute: true})
	if code != report.ExitEnvironment || result.OK || !hasDiagnostic(result.Errors, "SOURCE_COPY_MANIFEST_MISSING") {
		t.Fatalf("execute without manifest: code=%d result=%#v", code, result)
	}
	if _, err := os.Stat(filepath.Join(tasks, "NOMANIFEST")); !os.IsNotExist(err) {
		t.Fatalf("failed create created task directory: %v", err)
	}
	worktrees, err := service.Git.Worktrees(context.Background(), repo)
	if err != nil || len(worktrees) != 1 {
		t.Fatalf("failed create registered a worktree: err=%v worktrees=%#v", err, worktrees)
	}
}

func TestCreateRejectsInvalidManifest(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".worktreeinclude"), []byte("!keep.txt\n"), 0644); err != nil {
		t.Fatal(err)
	}
	preview, code := New().Create(context.Background(), CreateOptions{TasksRoot: t.TempDir(), TaskID: "BADMANIFEST", Repositories: []string{"app=" + repo}, DryRun: true})
	if code != report.ExitEnvironment || preview.OK || !hasDiagnostic(preview.Errors, "SOURCE_COPY_MANIFEST_INVALID") {
		t.Fatalf("invalid manifest: code=%d result=%#v", code, preview)
	}
}

func TestCreateWarnsOnUnmatchedLiteralPattern(t *testing.T) {
	repo := makeGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "present.env"), []byte("present"), 0600); err != nil {
		t.Fatal(err)
	}
	writeCopyManifest(t, repo, "present.env", "missing.env", "missing-dir/")
	tasks := t.TempDir()
	result, code := New().Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "WARN", Repositories: []string{"app=" + repo}, Execute: true})
	if code != report.ExitOK || !result.OK {
		t.Fatalf("create with unmatched literals: code=%d result=%#v", code, result)
	}
	warned := 0
	for _, warning := range result.Warnings {
		if warning.Code != "SOURCE_COPY_PATTERN_UNMATCHED" || warning.Repo != "app" {
			t.Fatalf("unexpected warning: %#v", warning)
		}
		if strings.Contains(warning.Message, "missing.env") || strings.Contains(warning.Message, "missing-dir/") {
			warned++
		}
	}
	if warned != 2 {
		t.Fatalf("expected two unmatched-pattern warnings, got %d in %#v", warned, result.Warnings)
	}
	if _, err := os.Stat(filepath.Join(tasks, "WARN", "worktrees", "app", "present.env")); err != nil {
		t.Fatalf("matched file missing: %v", err)
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

func hasDiagnostic(diagnostics []report.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func diagnosticHint(diagnostics []report.Diagnostic, code string) string {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return diagnostic.Hint
		}
	}
	return ""
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

func containsArg(args []string, expected string) bool {
	for _, arg := range args {
		if arg == expected {
			return true
		}
	}
	return false
}
