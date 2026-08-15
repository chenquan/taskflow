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
	root.SetArgs([]string{"create", "TASK-1", "--repo", "repo=" + repo, "--execute"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "TASK-1", "taskflow.yaml")); err != nil {
		t.Fatalf("task workspace was not created under current directory: %v", err)
	}
}

func TestSkillTargetsSelectTools(t *testing.T) {
	projectRoot := t.TempDir()
	t.Chdir(projectRoot)

	tests := []struct {
		name     string
		selected []string
		want     []string
	}{
		{name: "defaults to both", want: []string{"codex", "claude"}},
		{name: "codex only", selected: []string{"codex"}, want: []string{"codex"}},
		{name: "claude only", selected: []string{"claude"}, want: []string{"claude"}},
		{name: "repeatable and deduplicated", selected: []string{"codex", "claude", "codex"}, want: []string{"codex", "claude"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets, err := skillTargets(tt.selected, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(targets) != len(tt.want) {
				t.Fatalf("got %d targets, want %d: %#v", len(targets), len(tt.want), targets)
			}
			for i, wantTool := range tt.want {
				if targets[i].Tool != wantTool {
					t.Errorf("target %d tool=%q, want %q", i, targets[i].Tool, wantTool)
				}
				wantRoot := filepath.Join(projectRoot, "."+wantTool, "skills")
				if targets[i].Root != wantRoot {
					t.Errorf("target %d root=%q, want %q", i, targets[i].Root, wantRoot)
				}
			}
		})
	}
}

func TestSkillTargetsRejectUnknownTool(t *testing.T) {
	if _, err := skillTargets([]string{"windsurf"}, true); err == nil || !strings.Contains(err.Error(), "codex or claude") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSkillTargetsUseToolSpecificGlobalRoots(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, "codex-home")
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", codexHome)

	targets, err := skillTargets([]string{"codex", "claude"}, false)
	if err != nil {
		t.Fatal(err)
	}
	wantRoots := []string{
		filepath.Join(codexHome, "skills"),
		filepath.Join(home, ".claude", "skills"),
	}
	if len(targets) != len(wantRoots) {
		t.Fatalf("got %d targets, want %d: %#v", len(targets), len(wantRoots), targets)
	}
	for i, wantRoot := range wantRoots {
		if targets[i].Root != wantRoot {
			t.Errorf("target %d root=%q, want %q", i, targets[i].Root, wantRoot)
		}
	}
}

func TestPublicCommandsAreLimitedToCreateOpenVersionAndSkill(t *testing.T) {
	root := NewRootCommand()
	seen := map[string]bool{}
	for _, command := range root.Commands() {
		seen[command.Name()] = true
	}
	for _, name := range []string{"create", "open", "version", "skill"} {
		if !seen[name] {
			t.Fatalf("missing public command %s", name)
		}
	}
	for _, name := range []string{"init", "start", "status", "validate", "repo"} {
		if seen[name] {
			t.Fatalf("retired command still registered: %s", name)
		}
	}
}
