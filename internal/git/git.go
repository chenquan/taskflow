package git

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenquan/taskflow/internal/execx"
)

type Info struct {
	Root, CommonDir, Remote, DefaultBranch string
	Branch, Head, Upstream                 string
	Dirty                                  bool
	DirtyFiles, Ahead, Behind              int
}
type Worktree struct{ Path, Branch, Root string }
type Client struct{ Runner execx.Runner }

func (c Client) Inspect(ctx context.Context, path string) (Info, error) {
	r, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "rev-parse", "--show-toplevel"}})
	if err != nil {
		return Info{}, fmt.Errorf("not a git worktree: %s", strings.TrimSpace(r.Stderr))
	}
	info := Info{Root: strings.TrimSpace(r.Stdout)}
	common, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "rev-parse", "--git-common-dir"}})
	if err == nil {
		info.CommonDir = resolveGitPath(info.Root, strings.TrimSpace(common.Stdout))
	}
	s, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "status", "--porcelain=v1", "-z"}})
	if err == nil {
		info.Dirty = s.Stdout != ""
		info.DirtyFiles = dirtyRecordCount(s.Stdout)
	}
	remote, _ := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "remote", "get-url", "origin"}})
	info.Remote = strings.TrimSpace(remote.Stdout)
	head, _ := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"}})
	info.DefaultBranch = strings.TrimPrefix(strings.TrimSpace(head.Stdout), "origin/")
	branch, _ := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "branch", "--show-current"}})
	info.Branch = strings.TrimSpace(branch.Stdout)
	commit, _ := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "rev-parse", "HEAD"}})
	info.Head = strings.TrimSpace(commit.Stdout)
	upstream, upstreamErr := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"}})
	if upstreamErr == nil {
		info.Upstream = strings.TrimSpace(upstream.Stdout)
		counts, countErr := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}"}})
		if countErr == nil {
			fields := strings.Fields(counts.Stdout)
			if len(fields) == 2 {
				info.Ahead, _ = strconv.Atoi(fields[0])
				info.Behind, _ = strconv.Atoi(fields[1])
			}
		}
	}
	return info, nil
}

func dirtyRecordCount(output string) int {
	records := strings.Split(output, "\x00")
	count := 0
	for index := 0; index < len(records); index++ {
		record := records[index]
		if len(record) < 3 {
			continue
		}
		count++
		if record[0] == 'R' || record[0] == 'C' || record[1] == 'R' || record[1] == 'C' {
			index++
		}
	}
	return count
}

func resolveGitPath(root, value string) string {
	if value == "" {
		return ""
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(root, value)
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return filepath.Clean(value)
	}
	return filepath.Clean(absolute)
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

// ResetIndex populates a worktree index from HEAD without touching working
// tree files. `git worktree add --no-checkout` leaves the index empty, so a
// mixed reset is required before copied files can be compared against the
// configured base.
func (c Client) ResetIndex(ctx context.Context, path string) error {
	_, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "reset", "--quiet"}})
	return err
}

func gitCommandError(prefix string, result execx.Result) error {
	message := strings.TrimSpace(result.Stderr)
	if message == "" {
		return fmt.Errorf("%s", prefix)
	}
	return fmt.Errorf("%s: %s", prefix, message)
}

// DefaultBase resolves the source repository's remote default branch without
// fetching or changing Git state.
func (c Client) DefaultBase(ctx context.Context, path string) (string, error) {
	head, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"}})
	if err != nil {
		return "", fmt.Errorf("origin/HEAD is not configured")
	}
	ref := strings.TrimSpace(head.Stdout)
	if !strings.HasPrefix(ref, "origin/") || strings.TrimPrefix(ref, "origin/") == "" {
		return "", fmt.Errorf("origin/HEAD resolves to invalid reference %q", ref)
	}
	if !c.HasRef(ctx, path, ref) {
		return "", fmt.Errorf("remote default reference %s is unavailable locally", ref)
	}
	return ref, nil
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
// AddWorktree registers a worktree from the configured base. With noCheckout
// the working tree and index stay empty so the caller can populate the index
// and copy a source snapshot without overwriting base files.
func (c Client) AddWorktree(ctx context.Context, source, branch, target, base string, trackBase, noCheckout bool) error {
	args := []string{"-C", source, "worktree", "add"}
	if noCheckout {
		args = append(args, "--no-checkout")
	}
	if c.HasRef(ctx, source, "refs/heads/"+branch) {
		args = append(args, target, branch)
	} else {
		startingPoint := base
		if !trackBase {
			commit, err := c.ResolveCommit(ctx, source, base)
			if err != nil {
				return err
			}
			startingPoint = commit
		}
		args = append(args, "-b", branch, target, startingPoint)
	}
	_, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: args})
	return err
}

func (c Client) ResolveCommit(ctx context.Context, path, ref string) (string, error) {
	r, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", path, "rev-parse", "--verify", ref + "^{commit}"}})
	if err != nil {
		return "", fmt.Errorf("resolve base %s: %s", ref, strings.TrimSpace(r.Stderr))
	}
	commit := strings.TrimSpace(r.Stdout)
	if commit == "" {
		return "", fmt.Errorf("resolve base %s: empty commit result", ref)
	}
	return commit, nil
}

func (c Client) RemoveWorktree(ctx context.Context, source, target string, force bool) error {
	args := []string{"-C", source, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, target)
	_, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: args})
	return err
}

func (c Client) DeleteBranch(ctx context.Context, source, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "git", Args: []string{"-C", source, "branch", flag, branch}})
	return err
}
