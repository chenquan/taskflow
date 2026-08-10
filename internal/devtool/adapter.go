package devtool

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chenquan/taskflow/internal/domain"
)

type LaunchSpec struct {
	Executable, Dir string
	Args, Env       []string
}
type Adapter interface {
	Build(domain.Task, []string) (LaunchSpec, error)
}
type AdapterImpl struct{ Tool string }

func (a AdapterImpl) Build(t domain.Task, extraArgs []string) (LaunchSpec, error) {
	if a.Tool != "codex" && a.Tool != "claude" {
		return LaunchSpec{}, fmt.Errorf("unsupported tool %s", a.Tool)
	}
	if len(t.Repositories) == 0 {
		return LaunchSpec{}, fmt.Errorf("task has no repositories")
	}
	primary := filepath.Join(t.Task.Root, t.Repositories[0].Worktree)
	spec := LaunchSpec{Executable: a.Tool, Dir: primary}
	for _, r := range t.Repositories[1:] {
		p := filepath.Join(t.Task.Root, r.Worktree)
		spec.Args = append(spec.Args, "--add-dir", p)
	}
	spec.Args = append(spec.Args, "--add-dir", t.Task.Root)
	spec.Args = append(spec.Args, extraArgs...)
	if a.Tool == "claude" {
		spec.Env = []string{"CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1"}
	}
	for _, arg := range spec.Args {
		if arg == "--worktree" || strings.HasPrefix(arg, "--worktree=") {
			return LaunchSpec{}, fmt.Errorf("unsafe launch argument")
		}
	}
	return spec, nil
}

func ApplyEnv(base []string, overlay []string) []string {
	return append(append([]string{}, base...), overlay...)
}
