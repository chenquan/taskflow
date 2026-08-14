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
		{"-C", repo, "commit", "--allow-empty", "-m", "init"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	root := NewRootCommand()
	root.SetOut(os.Stdout)
	root.SetArgs([]string{"create", "TASK-1", "--repo", "repo=" + repo, "--execute"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "TASK-1", "taskflow.yaml")); err != nil {
		t.Fatalf("task workspace was not created under current directory: %v", err)
	}
}

func TestPublicCommandsAreLimitedToCreateOpenAndVersion(t *testing.T) {
	root := NewRootCommand()
	seen := map[string]bool{}
	for _, command := range root.Commands() {
		seen[command.Name()] = true
	}
	for _, name := range []string{"create", "open", "version"} {
		if !seen[name] {
			t.Fatalf("missing public command %s", name)
		}
	}
	for _, name := range []string{"init", "start", "status", "validate", "repo", "skill"} {
		if seen[name] {
			t.Fatalf("retired command still registered: %s", name)
		}
	}
}
