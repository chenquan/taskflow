package openspec

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chenquan/specflow/internal/execx"
)

type Client struct{ Runner execx.Runner }

type CompatibilityError struct{ Message string }

func (e CompatibilityError) Error() string { return e.Message }

type Artifact struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type Status struct {
	ChangeName string     `json:"changeName"`
	SchemaName string     `json:"schemaName"`
	IsComplete bool       `json:"isComplete"`
	Artifacts  []Artifact `json:"artifacts"`
}

type Validation struct {
	Valid  bool
	Issues []string
}

func (c Client) Available() bool { _, err := c.Runner.LookPath("openspec"); return err == nil }
func (c Client) ChangeExists(worktree, change string) bool {
	_, err := os.Stat(filepath.Join(worktree, "openspec", "changes", change))
	return err == nil
}
func (c Client) ChangeComplete(worktree, change string) (bool, error) {
	complete, total, err := c.TasksProgress(worktree, change)
	return err == nil && complete == total, err
}
func (c Client) TasksProgress(worktree, change string) (int, int, error) {
	tasksPath := filepath.Join(worktree, "openspec", "changes", change, "tasks.md")
	tasks, err := os.ReadFile(tasksPath)
	if err != nil {
		return 0, 0, fmt.Errorf("read OpenSpec tasks: %w", err)
	}
	complete, total := 0, 0
	scanner := bufio.NewScanner(strings.NewReader(string(tasks)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "- [ ]") {
			total++
		} else if strings.HasPrefix(line, "- [x]") || strings.HasPrefix(line, "- [X]") {
			total++
			complete++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, fmt.Errorf("read OpenSpec tasks: %w", err)
	}
	return complete, total, nil
}

func (c Client) Status(ctx context.Context, worktree, change string) (Status, error) {
	r, err := c.Runner.Run(ctx, execx.CommandSpec{Executable: "openspec", Args: []string{"status", "--change", change, "--json"}, Dir: worktree})
	if err != nil {
		return Status{}, fmt.Errorf("OpenSpec status %s: %s", change, strings.TrimSpace(r.Stderr))
	}
	var status Status
	if err := json.Unmarshal([]byte(r.Stdout), &status); err != nil {
		return Status{}, CompatibilityError{Message: fmt.Sprintf("decode OpenSpec status: %v", err)}
	}
	if status.ChangeName != change || status.SchemaName == "" || status.Artifacts == nil {
		return Status{}, CompatibilityError{Message: "OpenSpec status response is missing required fields"}
	}
	return status, nil
}

func (c Client) Validate(ctx context.Context, worktree, change string) (Validation, error) {
	r, runErr := c.Runner.Run(ctx, execx.CommandSpec{Executable: "openspec", Args: []string{"validate", change, "--strict", "--json", "--no-interactive"}, Dir: worktree})
	var envelope struct {
		Items []struct {
			ID     string `json:"id"`
			Valid  bool   `json:"valid"`
			Issues []any  `json:"issues"`
		} `json:"items"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(r.Stdout), &envelope); err != nil {
		message := fmt.Sprintf("decode OpenSpec validation: %v", err)
		if runErr != nil && strings.TrimSpace(r.Stderr) != "" {
			message += ": " + strings.TrimSpace(r.Stderr)
		}
		return Validation{}, CompatibilityError{Message: message}
	}
	if envelope.Version == "" || len(envelope.Items) == 0 {
		return Validation{}, CompatibilityError{Message: "OpenSpec validation response is missing required fields"}
	}
	for _, item := range envelope.Items {
		if item.ID != change {
			continue
		}
		issues := make([]string, 0, len(item.Issues))
		for _, issue := range item.Issues {
			encoded, _ := json.Marshal(issue)
			issues = append(issues, string(encoded))
		}
		return Validation{Valid: item.Valid, Issues: issues}, nil
	}
	return Validation{}, CompatibilityError{Message: "OpenSpec validation response does not contain the requested change"}
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
