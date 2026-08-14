package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateFlowRunsWithoutOpenSpec(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	for _, args := range [][]string{{"init", "-b", "main", repo}, {"-C", repo, "config", "user.email", "test@example.com"}, {"-C", repo, "config", "user.name", "Test"}, {"-C", repo, "commit", "--allow-empty", "-m", "init"}, {"-C", repo, "remote", "add", "origin", "https://example.test/repo.git"}, {"-C", repo, "update-ref", "refs/remotes/origin/main", "HEAD"}, {"-C", repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	tasks := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		root := NewRootCommand()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetErr(&output)
		root.SetArgs(append([]string{"--tasks-root", tasks}, args...))
		if err := root.Execute(); err != nil {
			t.Fatalf("taskflow %v: %v: %s", args, err, output.String())
		}
		return output.String()
	}
	preview := run("create", "task", "--repo", "repo="+repo, "--dry-run")
	if !strings.Contains(preview, "CREATE") {
		t.Fatalf("dry-run did not report create action: %s", preview)
	}
	if _, err := os.Stat(filepath.Join(tasks, "task", "taskflow.yaml")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote configuration: %v", err)
	}
	run("create", "task", "--repo", "repo="+repo, "--execute")
	if _, err := os.Stat(filepath.Join(tasks, "task", "worktrees", "repo")); err != nil {
		t.Fatalf("worktree missing: %v", err)
	}
	if raw, err := os.ReadFile(filepath.Join(tasks, "task", "taskflow.yaml")); err != nil || strings.Contains(string(raw), "openspec") {
		t.Fatalf("unexpected configuration: %s (%v)", raw, err)
	}
	run("create", "task", "--execute")
}
