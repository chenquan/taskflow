package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionUsesCobraCommand(t *testing.T) {
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), Version) {
		t.Fatalf("unexpected version output %q", out.String())
	}
}

func TestTasksRootDefaultsToCurrentDirectory(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	repo := filepath.Join(workspace, "repo")
	for _, args := range [][]string{
		{"init", "-b", "main", repo},
		{"-C", repo, "config", "user.email", "test@example.com"},
		{"-C", repo, "config", "user.name", "Test"},
		{"-C", repo, "commit", "--allow-empty", "-m", "init"},
		{"-C", repo, "remote", "add", "origin", "https://example.test/repo.git"},
		{"-C", repo, "update-ref", "refs/remotes/origin/main", "HEAD"},
		{"-C", repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	root := NewRootCommand()
	root.SetOut(os.Stdout)
	root.SetArgs([]string{"init", "TASK-1", "--repo", "repo=" + repo})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "TASK-1", "taskflow.yaml")); err != nil {
		t.Fatalf("task workspace was not created under current directory: %v", err)
	}
}

func TestSkillScope(t *testing.T) {
	if skillScope(true) != "project" || skillScope(false) != "global" {
		t.Fatal("unexpected skill scope")
	}
}

func TestCommandsReportRemovedConfiguration(t *testing.T) {
	tasks := t.TempDir()
	rootPath := filepath.Join(tasks, "LEGACY")
	if err := os.MkdirAll(rootPath, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(rootPath, "taskflow.yaml")
	raw := []byte("task:\n  id: LEGACY\nrepositories: []\ndevelopment:\n  default_tool: codex\n")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--json", "--tasks-root", tasks, "status", "LEGACY"})
	err := root.Execute()
	if exit, ok := err.(*exitError); !ok || exit.code != 2 || !strings.Contains(out.String(), "INVALID_CONFIGURATION") {
		t.Fatalf("err=%v output=%s", err, out.String())
	}
}
