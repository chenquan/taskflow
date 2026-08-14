package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/chenquan/taskflow/internal/app"
	"github.com/chenquan/taskflow/internal/report"
	"github.com/spf13/cobra"
)

var Version = "dev"

func Execute() {
	if err := NewRootCommand().Execute(); err != nil {
		if exit, ok := err.(*exitError); ok {
			os.Exit(exit.code)
		}
		os.Exit(1)
	}
}

func NewRootCommand() *cobra.Command {
	var tasksRoot = "."
	var asJSON bool
	svc := app.New()
	root := &cobra.Command{Use: "taskflow", Short: "Create Git worktrees and open AI coding tools", SilenceUsage: true}
	root.PersistentFlags().StringVar(&tasksRoot, "tasks-root", ".", "task workspace root (default: current directory)")
	root.PersistentFlags().BoolVar(&asJSON, "json", false, "emit JSON")
	render := func(c *cobra.Command, r report.Result, code report.ExitCode) error {
		if err := report.Render(c.OutOrStdout(), r, asJSON); err != nil {
			return err
		}
		if code != report.ExitOK {
			return &exitError{code: int(code)}
		}
		return nil
	}

	root.AddCommand(&cobra.Command{Use: "version", RunE: func(c *cobra.Command, args []string) error {
		_, err := fmt.Fprintln(c.OutOrStdout(), Version)
		return err
	}})

	var repositories []string
	var dryRun, execute bool
	create := &cobra.Command{Use: "create <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		r, code := svc.Create(context.Background(), app.CreateOptions{
			TasksRoot:    tasksRoot,
			TaskID:       args[0],
			Repositories: repositories,
			DryRun:       dryRun || !execute,
			Execute:      execute,
		})
		return render(c, r, code)
	}}
	create.Flags().StringSliceVar(&repositories, "repo", nil, "repository name=path (repeatable for a new task only)")
	create.Flags().BoolVar(&dryRun, "dry-run", false, "show the reconciliation plan without changing files or Git state (default)")
	create.Flags().BoolVar(&execute, "execute", false, "write initial taskflow.yaml and create missing worktrees")
	root.AddCommand(create)

	var tool string
	open := &cobra.Command{Use: "open <task-id> [-- <tool-args>...]", Args: cobra.MinimumNArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, err := svc.Load(tasksRoot, args[0])
		if err != nil {
			r := report.New("open", args[0])
			r.Fail(loadDiagnostic(err))
			return render(c, r, report.ExitConfig)
		}
		r, code := svc.Open(context.Background(), t, tool, args[1:], c.InOrStdin(), c.OutOrStdout(), c.ErrOrStderr())
		return render(c, r, code)
	}}
	open.Flags().StringVar(&tool, "tool", "", "codex or claude")
	root.AddCommand(open)
	return root
}

func loadDiagnostic(err error) report.Diagnostic {
	return report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()}
}

type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("command failed with exit code %d", e.code) }
