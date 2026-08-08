package devtool

import (
	"fmt"
	"github.com/chenquan/specflow/internal/domain"
	"os"
	"path/filepath"
	"strings"
)

type LaunchSpec struct {
	Executable, Dir string
	Args, Env       []string
}
type Adapter interface {
	Build(domain.Task) (LaunchSpec, error)
}
type AdapterImpl struct{ Tool string }

func (a AdapterImpl) Build(t domain.Task) (LaunchSpec, error) {
	if a.Tool != "codex" && a.Tool != "claude" {
		return LaunchSpec{}, fmt.Errorf("unsupported tool %s", a.Tool)
	}
	var primary string
	for _, r := range t.Repositories {
		if r.Name == t.Primary {
			primary = filepath.Join(t.Task.Root, r.Worktree)
		}
	}
	if primary == "" {
		return LaunchSpec{}, fmt.Errorf("primary repository is not configured")
	}
	spec := LaunchSpec{Executable: a.Tool, Dir: primary}
	for _, r := range t.Repositories {
		p := filepath.Join(t.Task.Root, r.Worktree)
		if p != primary {
			spec.Args = append(spec.Args, "--add-dir", p)
		}
	}
	spec.Args = append(spec.Args, "--add-dir", t.Task.Root)
	if a.Tool == "claude" && t.Development.Tools["claude"].LoadAdditionalInstructions {
		spec.Env = []string{"CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1"}
	}
	for _, arg := range spec.Args {
		if strings.Contains(arg, "dangerously") || arg == "--worktree" {
			return LaunchSpec{}, fmt.Errorf("unsafe launch argument")
		}
	}
	return spec, nil
}
func ApplyEnv(base []string, overlay []string) []string {
	return append(append([]string{}, base...), overlay...)
}

var _ = os.Environ
