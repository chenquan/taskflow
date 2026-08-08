package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/chenquan/specflow/internal/app"
	"github.com/chenquan/specflow/internal/report"
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
	var tasksRoot string
	var asJSON bool
	svc := app.New()
	root := &cobra.Command{Use: "specflow", Short: "Safely coordinate local multi-repository OpenSpec work", SilenceUsage: true}
	root.PersistentFlags().StringVar(&tasksRoot, "tasks-root", "", "task workspace root")
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
	var primary string
	var repos []string
	init := &cobra.Command{Use: "init <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		r, code := svc.Init(context.Background(), app.InitOptions{TasksRoot: tasksRoot, TaskID: args[0], Primary: primary, Repositories: repos})
		return render(c, r, code)
	}}
	init.Flags().StringVar(&primary, "primary", "", "primary repository name")
	init.Flags().StringSliceVar(&repos, "repo", nil, "repository name=path (repeatable)")
	root.AddCommand(init)
	var dryRun, execute bool
	start := &cobra.Command{Use: "start <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, err := svc.Load(tasksRoot, args[0])
		if err != nil {
			r := report.New("start", args[0])
			r.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()})
			return render(c, r, report.ExitConfig)
		}
		if dryRun && execute {
			r := report.New("start", args[0])
			r.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: "--dry-run and --execute are mutually exclusive"})
			return render(c, r, report.ExitConfig)
		}
		r, code := svc.Start(context.Background(), t, app.StartOptions{DryRun: !execute, Execute: execute})
		return render(c, r, code)
	}}
	start.Flags().BoolVar(&dryRun, "dry-run", false, "show the execution plan without changing state")
	start.Flags().BoolVar(&execute, "execute", false, "execute the planned worktree and OpenSpec actions")
	root.AddCommand(start)
	var tool string
	openCmd := &cobra.Command{Use: "open <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, e := svc.Load(tasksRoot, args[0])
		if e != nil {
			r := report.New("open", args[0])
			r.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: e.Error()})
			return render(c, r, report.ExitConfig)
		}
		r, code := svc.Open(context.Background(), t, tool, c.InOrStdin(), c.OutOrStdout(), c.ErrOrStderr())
		return render(c, r, code)
	}}
	openCmd.Flags().StringVar(&tool, "tool", "", "codex or claude")
	root.AddCommand(openCmd)
	root.AddCommand(&cobra.Command{Use: "status <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, e := svc.Load(tasksRoot, args[0])
		if e != nil {
			r := report.New("status", args[0])
			r.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: e.Error()})
			return render(c, r, report.ExitConfig)
		}
		r, code := svc.Status(context.Background(), t)
		return render(c, r, code)
	}})
	var validateRepo string
	validate := &cobra.Command{Use: "validate <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, e := svc.Load(tasksRoot, args[0])
		if e != nil {
			r := report.New("validate", args[0])
			r.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: e.Error()})
			return render(c, r, report.ExitConfig)
		}
		r, code := svc.ValidateScoped(context.Background(), t, validateRepo)
		return render(c, r, code)
	}}
	validate.Flags().StringVar(&validateRepo, "repo", "", "validate this repository and its dependencies")
	root.AddCommand(validate)
	var finishDryRun bool
	finish := &cobra.Command{Use: "finish <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		if !finishDryRun {
			r := report.New("finish", args[0])
			r.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: "finish requires --dry-run"})
			return render(c, r, report.ExitConfig)
		}
		t, e := svc.Load(tasksRoot, args[0])
		if e != nil {
			r := report.New("finish", args[0])
			r.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: e.Error()})
			return render(c, r, report.ExitConfig)
		}
		r, code := svc.Finish(context.Background(), t)
		return render(c, r, code)
	}}
	finish.Flags().BoolVar(&finishDryRun, "dry-run", false, "generate a non-mutating readiness report")
	root.AddCommand(finish)
	configCmd := &cobra.Command{Use: "config"}
	configCmd.AddCommand(&cobra.Command{Use: "show <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, err := svc.Load(tasksRoot, args[0])
		r := report.New("config show", args[0])
		if err != nil {
			r.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()})
			return render(c, r, report.ExitConfig)
		}
		r.Data = t
		return render(c, r, report.ExitOK)
	}})
	configCmd.AddCommand(&cobra.Command{Use: "validate <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, err := svc.Load(tasksRoot, args[0])
		if err != nil {
			r := report.New("config validate", args[0])
			r.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()})
			return render(c, r, report.ExitConfig)
		}
		r, code := svc.ConfigValidate(context.Background(), t)
		return render(c, r, code)
	}})
	root.AddCommand(configCmd)
	var repo string
	doctor := &cobra.Command{Use: "doctor <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, err := svc.Load(tasksRoot, args[0])
		if err != nil {
			r := report.New("doctor", args[0])
			r.Fail(report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()})
			return render(c, r, report.ExitConfig)
		}
		r, code := svc.Doctor(context.Background(), t, repo)
		return render(c, r, code)
	}}
	doctor.Flags().StringVar(&repo, "repo", "", "only diagnose this repository")
	root.AddCommand(doctor)
	return root
}

type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("command failed with exit code %d", e.code) }
