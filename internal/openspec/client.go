package openspec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenquan/specflow/internal/execx"
)

type Client struct{ Runner execx.Runner }

func (c Client) Available() bool { _, err := c.Runner.LookPath("openspec"); return err == nil }
func (c Client) ChangeExists(worktree, change string) bool {
	_, err := os.Stat(filepath.Join(worktree, "openspec", "changes", change))
	return err == nil
}
func (c Client) ChangeComplete(worktree, change string) (bool, error) {
	tasksPath := filepath.Join(worktree, "openspec", "changes", change, "tasks.md")
	tasks, err := os.ReadFile(tasksPath)
	if err != nil {
		return false, fmt.Errorf("read OpenSpec tasks: %w", err)
	}
	return !bytes.Contains(tasks, []byte("- [ ]")), nil
}
func (c Client) CreateChange(ctx context.Context, worktree, change string) error {
	r, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "openspec", Args: []string{"new", "change", change, "--json"}, Dir: worktree})
	if err != nil {
		return fmt.Errorf("create OpenSpec change %s: %s", change, r.Stderr)
	}
	if !c.ChangeExists(worktree, change) {
		return fmt.Errorf("OpenSpec did not create change directory %s", change)
	}
	return nil
}
