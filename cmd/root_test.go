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
		{"init", repo},
		{"-C", repo, "config", "user.email", "test@example.com"},
		{"-C", repo, "config", "user.name", "Test"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	root := NewRootCommand()
	root.SetOut(os.Stdout)
	root.SetArgs([]string{"init", "TASK-1", "--primary", "repo", "--repo", "repo=" + repo})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "TASK-1", "specflow.yaml")); err != nil {
		t.Fatalf("task workspace was not created under current directory: %v", err)
	}
}
