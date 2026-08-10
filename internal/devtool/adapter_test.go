package devtool

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chenquan/taskflow/internal/domain"
)

func TestLaunchSpecUsesBuiltInToolsAndRepositoryOrder(t *testing.T) {
	root := filepath.FromSlash("/tmp/task")
	task := domain.Task{Task: domain.TaskInfo{Root: root}, Repositories: []domain.Repository{{Name: "owner", Worktree: "worktrees/owner"}, {Name: "sdk", Worktree: "worktrees/sdk"}, {Name: "ui", Worktree: "worktrees/ui"}}}

	codex, err := (AdapterImpl{Tool: "codex"}).Build(task, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"--add-dir", filepath.Join(root, "worktrees", "sdk"), "--add-dir", filepath.Join(root, "worktrees", "ui"), "--add-dir", root}
	if codex.Executable != "codex" || codex.Dir != filepath.Join(root, "worktrees", "owner") || !reflect.DeepEqual(codex.Args, wantArgs) || len(codex.Env) != 0 {
		t.Fatalf("codex: %#v", codex)
	}

	claude, err := (AdapterImpl{Tool: "claude"}).Build(task, nil)
	if err != nil {
		t.Fatal(err)
	}
	if claude.Executable != "claude" || !strings.Contains(strings.Join(claude.Env, " "), "CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1") {
		t.Fatalf("claude: %#v", claude)
	}
}

func TestLaunchSpecRejectsUnsupportedToolsAndMissingRepositories(t *testing.T) {
	root := filepath.FromSlash("/tmp/task")
	if _, err := (AdapterImpl{Tool: "other"}).Build(domain.Task{Task: domain.TaskInfo{Root: root}, Repositories: []domain.Repository{{Name: "repo"}}}, nil); err == nil {
		t.Fatal("expected unsupported tool error")
	}
	if _, err := (AdapterImpl{Tool: "codex"}).Build(domain.Task{Task: domain.TaskInfo{Root: root}}, nil); err == nil {
		t.Fatal("expected missing repository error")
	}
}

func TestApplyEnvPreservesBaseAndOverlay(t *testing.T) {
	base := []string{"A=1"}
	got := ApplyEnv(base, []string{"B=2"})
	if !reflect.DeepEqual(got, []string{"A=1", "B=2"}) || len(base) != 1 {
		t.Fatalf("base=%v got=%v", base, got)
	}
}

func TestBuildAppendsExtraArgsAndRejectsNestedWorktrees(t *testing.T) {
	root := filepath.FromSlash("/tmp/task")
	task := domain.Task{Task: domain.TaskInfo{Root: root}, Repositories: []domain.Repository{{Name: "repo", Worktree: "worktrees/repo"}}}
	spec, err := AdapterImpl{Tool: "codex"}.Build(task, []string{"--model", "gpt"})
	if err != nil {
		t.Fatal(err)
	}
	last := spec.Args[len(spec.Args)-2:]
	if last[0] != "--model" || last[1] != "gpt" {
		t.Fatalf("extra args not appended: %#v", spec.Args)
	}
	spec, err = (AdapterImpl{Tool: "codex"}).Build(task, []string{"--dangerously-skip-permissions"})
	if err != nil || spec.Args[len(spec.Args)-1] != "--dangerously-skip-permissions" {
		t.Fatalf("permission argument should be forwarded: %#v, %v", spec, err)
	}
	for _, args := range [][]string{{"--worktree", "other"}, {"--worktree=other"}} {
		if _, err := (AdapterImpl{Tool: "codex"}).Build(task, args); err == nil {
			t.Fatalf("expected nested worktree argument %v to be rejected", args)
		}
	}
}
