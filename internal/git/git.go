package git

import (
	"context"
	"fmt"
	"github.com/chenquan/specflow/internal/execx"
	"os"
	"path/filepath"
	"strings"
)

type Info struct {
	Root, Remote, DefaultBranch string
	Branch                      string
	Dirty                       bool
}
type Worktree struct{ Path, Branch, Root string }
type Client struct{ Runner execx.Runner }

func (c Client) Inspect(ctx context.Context, path string) (Info, error) {
	r, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "rev-parse", "--show-toplevel"}})
	if err != nil {
		return Info{}, fmt.Errorf("not a git worktree: %s", strings.TrimSpace(r.Stderr))
	}
	info := Info{Root: strings.TrimSpace(r.Stdout)}
	s, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "status", "--porcelain"}})
	if err == nil {
		info.Dirty = strings.TrimSpace(s.Stdout) != ""
	}
	remote, _ := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "remote", "get-url", "origin"}})
	info.Remote = strings.TrimSpace(remote.Stdout)
	head, _ := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"}})
	info.DefaultBranch = strings.TrimPrefix(strings.TrimSpace(head.Stdout), "origin/")
	branch, _ := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "branch", "--show-current"}})
	info.Branch = strings.TrimSpace(branch.Stdout)
	return info, nil
}
func (c Client) RemoteExists(ctx context.Context, path, remote string) bool {
	_, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "remote", "get-url", remote}})
	return err == nil
}
func (c Client) Fetch(ctx context.Context, path, remote string) error {
	_, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "fetch", remote}})
	return err
}
func (c Client) HasRef(ctx context.Context, path, ref string) bool {
	_, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "rev-parse", "--verify", "--quiet", ref + "^{commit}"}})
	return err == nil
}
func IsOpenSpec(path string) bool {
	s, e := os.Stat(filepath.Join(path, "openspec"))
	return e == nil && s.IsDir()
}

func (c Client) Worktrees(ctx context.Context, path string) ([]Worktree, error) {
	r, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "worktree", "list", "--porcelain"}})
	if err != nil {
		return nil, err
	}
	var result []Worktree
	var current Worktree
	flush := func() {
		if current.Path != "" {
			result = append(result, current)
		}
		current = Worktree{}
	}
	for _, line := range strings.Split(r.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			flush()
			continue
		}
		switch fields[0] {
		case "worktree":
			if current.Path != "" {
				flush()
			}
			if len(fields) > 1 {
				current.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			}
		case "branch":
			if len(fields) > 1 {
				current.Branch = strings.TrimPrefix(fields[1], "refs/heads/")
			}
		case "bare":
			current.Root = current.Path
		}
	}
	flush()
	return result, nil
}
func (c Client) AddWorktree(ctx context.Context, source, branch, target, base string) error {
	_, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", source, "worktree", "add", "-b", branch, target, base}})
	return err
}
