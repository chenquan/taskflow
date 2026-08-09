package devtool

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chenquan/specflow/internal/domain"
)

func TestLaunchSpecSafety(t *testing.T) {
	root := filepath.FromSlash("/tmp/task")
	task := domain.Task{Task: domain.TaskInfo{Root: root}, Primary: "a", Repositories: []domain.Repository{{Name: "a", Worktree: "worktrees/a"}, {Name: "b", Worktree: "worktrees/b"}}, Development: domain.Development{Tools: map[string]domain.ToolDef{"claude": {Executable: "custom-claude", LoadAdditionalInstructions: true}}}}
	s, e := AdapterImpl{Tool: "claude"}.Build(task)
	if e != nil {
		t.Fatal(e)
	}
	if s.Executable != "custom-claude" || s.Dir != filepath.Join(root, "worktrees", "a") || len(s.Args) != 4 || !strings.Contains(strings.Join(s.Env, " "), "CLAUDE_CODE") {
		t.Fatalf("%#v", s)
	}
}

func TestCodexUsesConfiguredExecutableAndAdditionalDirectories(t *testing.T) {
	root := filepath.FromSlash("/tmp/task")
	task := domain.Task{Task: domain.TaskInfo{Root: root}, Primary: "owner", Repositories: []domain.Repository{{Name: "owner", Worktree: "worktrees/owner"}, {Name: "sdk", Worktree: "worktrees/sdk"}, {Name: "ui", Worktree: "worktrees/ui"}}, Development: domain.Development{Tools: map[string]domain.ToolDef{"codex": {Executable: "/opt/tools/custom-codex"}}}}
	spec, err := (AdapterImpl{Tool: "codex"}).Build(task)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--add-dir", filepath.Join(root, "worktrees", "sdk"), "--add-dir", filepath.Join(root, "worktrees", "ui"), "--add-dir", root}
	if spec.Executable != "/opt/tools/custom-codex" || spec.Dir != filepath.Join(root, "worktrees", "owner") || !reflect.DeepEqual(spec.Args, wantArgs) {
		t.Fatalf("%#v", spec)
	}
}

func TestLaunchSpecRejectsInvalidPolicies(t *testing.T) {
	root := filepath.FromSlash("/tmp/task")
	base := domain.Task{Task: domain.TaskInfo{Root: root}, Primary: "repo", Repositories: []domain.Repository{{Name: "repo", Worktree: "worktrees/repo"}}, Development: domain.Development{Tools: map[string]domain.ToolDef{"codex": {Executable: "codex"}}}}
	cases := []struct {
		name string
		tool string
		edit func(*domain.Task)
	}{
		{name: "unsupported", tool: "other", edit: func(*domain.Task) {}},
		{name: "missing definition", tool: "codex", edit: func(task *domain.Task) { delete(task.Development.Tools, "codex") }},
		{name: "missing primary", tool: "codex", edit: func(task *domain.Task) { task.Primary = "missing" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := base
			task.Development.Tools = map[string]domain.ToolDef{"codex": base.Development.Tools["codex"]}
			tc.edit(&task)
			if _, err := (AdapterImpl{Tool: tc.tool}).Build(task); err == nil {
				t.Fatal("expected policy error")
			}
		})
	}
}

func TestApplyEnvPreservesBaseAndOverlay(t *testing.T) {
	base := []string{"A=1"}
	got := ApplyEnv(base, []string{"B=2"})
	if !reflect.DeepEqual(got, []string{"A=1", "B=2"}) || len(base) != 1 {
		t.Fatalf("base=%v got=%v", base, got)
	}
}
