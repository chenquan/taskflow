package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenquan/taskflow/internal/config"
	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/execx"
	"github.com/chenquan/taskflow/internal/git"
	"github.com/chenquan/taskflow/internal/lock"
)

type resultEnvelope struct {
	OK     bool              `json:"ok"`
	Data   json.RawMessage   `json:"data"`
	Errors []json.RawMessage `json:"errors"`
}

func e2eGitRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	for _, args := range [][]string{{"init", "-b", "main", dir}, {"-C", dir, "config", "user.email", "test@example.com"}, {"-C", dir, "config", "user.name", "Test"}, {"-C", dir, "commit", "--allow-empty", "-m", "init"}, {"-C", dir, "remote", "add", "origin", "https://example.test/repo.git"}, {"-C", dir, "update-ref", "refs/remotes/origin/main", "HEAD"}, {"-C", dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

func runE2E(t *testing.T, tasks string, args ...string) (string, error) {
	t.Helper()
	root := NewRootCommand()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(append([]string{"--tasks-root", tasks}, args...))
	err := root.Execute()
	return output.String(), err
}

func TestE2ECreateJSONAndReuseDirtyWorktree(t *testing.T) {
	repo1, repo2 := e2eGitRepo(t), e2eGitRepo(t)
	tasks := t.TempDir()
	preview, err := runE2E(t, tasks, "--json", "create", "FLOW", "--repo", "one="+repo1, "--repo", "two="+repo2, "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v: %s", err, preview)
	}
	var result resultEnvelope
	if err := json.Unmarshal([]byte(preview), &result); err != nil || !result.OK {
		t.Fatalf("dry-run JSON: %v %s", err, preview)
	}
	if _, err := os.Stat(filepath.Join(tasks, "FLOW", "taskflow.yaml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote taskflow.yaml: %v", err)
	}
	if _, err := runE2E(t, tasks, "--json", "create", "FLOW", "--repo", "one="+repo1, "--repo", "two="+repo2, "--execute"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	root := filepath.Join(tasks, "FLOW")
	for _, name := range []string{"state.json", "inventory.json", filepath.Join("reports", "validation.json")} {
		if _, err := os.Stat(filepath.Join(root, ".taskflow", name)); !os.IsNotExist(err) {
			t.Fatalf("legacy file exists %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "worktrees", "one", "dirty.txt"), []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}
	reuseOutput, err := runE2E(t, tasks, "--json", "create", "FLOW", "--dry-run")
	if err != nil {
		t.Fatalf("reuse dry-run: %v: %s", err, reuseOutput)
	}
	var reuse resultEnvelope
	if err := json.Unmarshal([]byte(reuseOutput), &reuse); err != nil || !reuse.OK {
		t.Fatalf("reuse dry-run JSON: %v %s", err, reuseOutput)
	}
	var reuseData struct {
		Actions []struct {
			Repo   string `json:"repo"`
			Kind   string `json:"kind"`
			Status string `json:"status"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(reuse.Data, &reuseData); err != nil {
		t.Fatalf("reuse data: %v", err)
	}
	if len(reuseData.Actions) != 4 || reuseData.Actions[0].Repo != "one" || reuseData.Actions[0].Kind != "worktree" || reuseData.Actions[0].Status != "reuse" || reuseData.Actions[1].Kind != "source-copy" || reuseData.Actions[1].Status != "reuse" || reuseData.Actions[2].Repo != "two" || reuseData.Actions[2].Kind != "worktree" || reuseData.Actions[2].Status != "reuse" || reuseData.Actions[3].Kind != "source-copy" || reuseData.Actions[3].Status != "reuse" {
		t.Fatalf("reuse actions: %#v", reuseData.Actions)
	}
	if _, err := runE2E(t, tasks, "--json", "create", "FLOW", "--execute"); err != nil {
		t.Fatalf("repeat create: %v", err)
	}
}

func TestE2ECreateConflictPreservesTarget(t *testing.T) {
	repo := e2eGitRepo(t)
	tasks := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tasks, "CONFLICT", "worktrees", "repo"), 0755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(tasks, "CONFLICT", "worktrees", "repo", "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	// The first dry-run must not mistake a pre-existing task directory for a
	// workspace that it may overwrite.
	if output, err := runE2E(t, tasks, "--json", "create", "CONFLICT", "--repo", "repo="+repo, "--execute"); err == nil || !strings.Contains(output, "UNMANAGED_TASK_DIRECTORY") {
		t.Fatalf("expected unmanaged directory rejection: err=%v output=%s", err, output)
	}
	if got, _ := os.ReadFile(keep); string(got) != "keep" {
		t.Fatal("conflict changed target")
	}
}

func TestE2ESourceBranchLockConflict(t *testing.T) {
	repo := e2eGitRepo(t)
	tasks := t.TempDir()
	if output, err := runE2E(t, tasks, "create", "LOCK", "--repo", "repo="+repo, "--execute"); err != nil {
		t.Fatalf("initial create: %v: %s", err, output)
	}
	info, err := (git.Client{Runner: gitRunner()}).Inspect(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	holder, err := lock.AcquireSource(info.CommonDir, "feature/lock")
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Release()
	output, err := runE2E(t, tasks, "--json", "create", "LOCK", "--execute")
	if err == nil || !strings.Contains(output, "SOURCE_BRANCH_LOCKED") {
		t.Fatalf("expected source lock conflict: err=%v output=%s", err, output)
	}
}

func TestE2EDirectConfigurationEditAndAppendRejection(t *testing.T) {
	repo1, repo2 := e2eGitRepo(t), e2eGitRepo(t)
	tasks := t.TempDir()
	if output, err := runE2E(t, tasks, "create", "EDIT", "--repo", "one="+repo1, "--execute"); err != nil {
		t.Fatalf("initial create: %v: %s", err, output)
	}

	configPath := filepath.Join(tasks, "EDIT", "taskflow.yaml")
	task, err := config.Load(configPath)
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
	if output, err := runE2E(t, tasks, "--json", "create", "EDIT", "--execute"); err != nil {
		t.Fatalf("direct config reconcile: %v: %s", err, output)
	}
	after, _ := os.ReadFile(configPath)
	if string(before) != string(after) {
		t.Fatal("direct config reconcile rewrote taskflow.yaml")
	}
	if _, err := os.Stat(filepath.Join(tasks, "EDIT", "worktrees", "two")); err != nil {
		t.Fatalf("directly declared worktree missing: %v", err)
	}

	if output, err := runE2E(t, tasks, "--json", "create", "EDIT", "--repo", "three="+e2eGitRepo(t), "--dry-run"); err == nil || !strings.Contains(output, "CONFIG_EDIT_REQUIRED") {
		t.Fatalf("expected CONFIG_EDIT_REQUIRED: err=%v output=%s", err, output)
	}
	if got, _ := os.ReadFile(configPath); string(got) != string(after) {
		t.Fatal("existing-task --repo changed taskflow.yaml")
	}

	task.Repositories = task.Repositories[:1]
	raw, err = config.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := runE2E(t, tasks, "create", "EDIT", "--execute"); err != nil {
		t.Fatalf("direct config removal reconcile: %v: %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(tasks, "EDIT", "worktrees", "two")); err != nil {
		t.Fatalf("unlisted worktree was removed: %v", err)
	}
}

func TestE2EDeleteOnlyRemovesOwnedTask(t *testing.T) {
	repo := e2eGitRepo(t)
	tasks := t.TempDir()
	if output, err := runE2E(t, tasks, "create", "DELETE", "--repo", "repo="+repo, "--execute"); err != nil {
		t.Fatalf("create: %v: %s", err, output)
	}
	taskRoot := filepath.Join(tasks, "DELETE")
	if _, err := os.Stat(filepath.Join(taskRoot, ".taskflow", "ownership.json")); err != nil {
		t.Fatalf("ownership manifest missing: %v", err)
	}
	preview, err := runE2E(t, tasks, "--json", "delete", "DELETE")
	if err != nil || !strings.Contains(preview, "REMOVE worktree") || !strings.Contains(preview, "DELETE local branch") {
		t.Fatalf("delete dry-run: %v: %s", err, preview)
	}
	if _, err := os.Stat(taskRoot); err != nil {
		t.Fatalf("dry-run removed task root: %v", err)
	}
	if output, err := runE2E(t, tasks, "--json", "delete", "DELETE", "--execute"); err != nil {
		t.Fatalf("delete execute: %v: %s", err, output)
	}
	if _, err := os.Stat(taskRoot); !os.IsNotExist(err) {
		t.Fatalf("task root remains: %v", err)
	}
	if output, err := exec.Command("git", "-C", repo, "rev-parse", "--verify", "refs/heads/feature/delete").CombinedOutput(); err == nil {
		t.Fatalf("task branch remains: %s", output)
	}
	worktrees, err := (git.Client{Runner: gitRunner()}).Worktrees(context.Background(), repo)
	if err != nil || len(worktrees) != 1 {
		t.Fatalf("source worktrees after delete: %v: %#v", err, worktrees)
	}
}

func TestE2EDeleteRefusesUnownedAndDirtyWorktrees(t *testing.T) {
	repo := e2eGitRepo(t)
	tasks := t.TempDir()
	if output, err := runE2E(t, tasks, "create", "UNOWNED", "--repo", "repo="+repo, "--execute"); err != nil {
		t.Fatalf("create: %v: %s", err, output)
	}
	taskRoot := filepath.Join(tasks, "UNOWNED")
	if err := os.Remove(filepath.Join(taskRoot, ".taskflow", "ownership.json")); err != nil {
		t.Fatal(err)
	}
	if output, err := runE2E(t, tasks, "--json", "delete", "UNOWNED", "--execute"); err == nil || !strings.Contains(output, "OWNERSHIP_NOT_FOUND") {
		t.Fatalf("expected ownership refusal: %v: %s", err, output)
	}
	if _, err := os.Stat(taskRoot); err != nil {
		t.Fatalf("unowned delete changed task root: %v", err)
	}

	if output, err := runE2E(t, tasks, "create", "DIRTY", "--repo", "repo="+repo, "--execute"); err != nil {
		t.Fatalf("second create: %v: %s", err, output)
	}
	dirtyTarget := filepath.Join(tasks, "DIRTY", "worktrees", "repo")
	if err := os.WriteFile(filepath.Join(dirtyTarget, "local.txt"), []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := runE2E(t, tasks, "--json", "delete", "DIRTY", "--execute"); err == nil || !strings.Contains(output, "WORKTREE_DIRTY") {
		t.Fatalf("expected dirty refusal: %v: %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(dirtyTarget, "local.txt")); err != nil {
		t.Fatalf("dirty file was removed: %v", err)
	}
	if output, err := runE2E(t, tasks, "--json", "delete", "DIRTY", "--execute", "--force"); err != nil {
		t.Fatalf("forced delete: %v: %s", err, output)
	}
}

func TestE2EDeleteTreatsCopiedSnapshotAsDirty(t *testing.T) {
	repo := e2eGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "local.env"), []byte("local"), 0600); err != nil {
		t.Fatal(err)
	}
	tasks := t.TempDir()
	if output, err := runE2E(t, tasks, "create", "SNAPSHOT-DELETE", "--repo", "repo="+repo, "--execute"); err != nil {
		t.Fatalf("snapshot create: %v: %s", err, output)
	}
	target := filepath.Join(tasks, "SNAPSHOT-DELETE", "worktrees", "repo")
	if _, err := os.Stat(filepath.Join(target, "local.env")); err != nil {
		t.Fatalf("copied snapshot is incomplete: %v", err)
	}
	if output, err := runE2E(t, tasks, "--json", "delete", "SNAPSHOT-DELETE", "--execute"); err == nil || !strings.Contains(output, "WORKTREE_DIRTY") {
		t.Fatalf("expected copied-snapshot dirty refusal: %v: %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(target, "local.env")); err != nil {
		t.Fatalf("snapshot file was removed by refused delete: %v", err)
	}
	if output, err := runE2E(t, tasks, "delete", "SNAPSHOT-DELETE", "--execute", "--force"); err != nil {
		t.Fatalf("forced delete: %v: %s", err, output)
	}
}

func TestE2EDeleteRefusesUnmanagedTaskFiles(t *testing.T) {
	repo := e2eGitRepo(t)
	tasks := t.TempDir()
	if output, err := runE2E(t, tasks, "create", "EXTRA", "--repo", "repo="+repo, "--execute"); err != nil {
		t.Fatalf("create: %v: %s", err, output)
	}
	taskRoot := filepath.Join(tasks, "EXTRA")
	note := filepath.Join(taskRoot, "notes.md")
	if err := os.WriteFile(note, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := runE2E(t, tasks, "--json", "delete", "EXTRA", "--execute"); err == nil || !strings.Contains(output, "DELETE_DIRECTORY_UNSAFE") {
		t.Fatalf("expected extra-file refusal: %v: %s", err, output)
	}
	if _, err := os.Stat(note); err != nil {
		t.Fatalf("extra file was removed: %v", err)
	}
}

func TestE2EDeleteRequiresKnownDefaultBranch(t *testing.T) {
	repo := e2eGitRepo(t)
	tasks := t.TempDir()
	if output, err := runE2E(t, tasks, "create", "NODEFAULT", "--repo", "repo="+repo, "--execute"); err != nil {
		t.Fatalf("create: %v: %s", err, output)
	}
	if output, err := exec.Command("git", "-C", repo, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD").CombinedOutput(); err != nil {
		t.Fatalf("remove origin/HEAD: %v: %s", err, output)
	}
	output, err := runE2E(t, tasks, "--json", "delete", "NODEFAULT", "--execute")
	if err == nil || !strings.Contains(output, "DEFAULT_BRANCH_UNKNOWN") {
		t.Fatalf("expected default branch refusal: %v: %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(tasks, "NODEFAULT")); err != nil {
		t.Fatalf("default branch refusal changed task root: %v", err)
	}
}

func gitRunner() execx.Runner { return execx.OSRunner{} }
