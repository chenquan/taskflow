package devtool

import (
	"github.com/chenquan/specflow/internal/domain"
	"strings"
	"testing"
)

func TestLaunchSpecSafety(t *testing.T) {
	task := domain.Task{Task: domain.TaskInfo{Root: "/tmp/task"}, Primary: "a", Repositories: []domain.Repository{{Name: "a", Worktree: "worktrees/a"}, {Name: "b", Worktree: "worktrees/b"}}, Development: domain.Development{Tools: map[string]domain.ToolDef{"claude": {LoadAdditionalInstructions: true}}}}
	s, e := AdapterImpl{Tool: "claude"}.Build(task)
	if e != nil {
		t.Fatal(e)
	}
	if s.Dir != "/tmp/task/worktrees/a" || len(s.Args) != 4 || !strings.Contains(strings.Join(s.Env, " "), "CLAUDE_CODE") {
		t.Fatalf("%#v", s)
	}
}
