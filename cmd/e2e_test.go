package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLifecycleRunsWithoutOpenSpec(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	for _, args := range [][]string{{"init", repo}, {"-C", repo, "config", "user.email", "test@example.com"}, {"-C", repo, "config", "user.name", "Test"}, {"-C", repo, "commit", "--allow-empty", "-m", "init"}} {
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
			t.Fatalf("specflow %v: %v: %s", args, err, output.String())
		}
		return output.String()
	}
	run("init", "task", "--primary", "repo", "--repo", "repo="+repo)
	raw, err := os.ReadFile(filepath.Join(tasks, "task", "specflow.yaml"))
	if err != nil || strings.Contains(string(raw), "openspec") {
		t.Fatalf("OpenSpec leaked into task configuration: %s (%v)", raw, err)
	}
	run("start", "task", "--execute")
	status := run("--json", "status", "task")
	if strings.Contains(status, "openSpec") || !strings.Contains(status, "feature/task") {
		t.Fatalf("unexpected status: %s", status)
	}
	run("validate", "task")
}
