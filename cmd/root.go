package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenquan/taskflow/internal/app"
	"github.com/chenquan/taskflow/internal/config"
	"github.com/chenquan/taskflow/internal/report"
	"github.com/chenquan/taskflow/skills"
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
	// Keep task workspaces local to the directory from which taskflow is run
	// unless the caller explicitly selects another root with --tasks-root.
	var tasksRoot = "."
	var asJSON bool
	svc := app.New()
	root := &cobra.Command{Use: "taskflow", Short: "Coordinate AI-assisted work across local Git repositories", SilenceUsage: true}
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
	var repos []string
	init := &cobra.Command{Use: "init <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		r, code := svc.Init(context.Background(), app.InitOptions{TasksRoot: tasksRoot, TaskID: args[0], Repositories: repos})
		return render(c, r, code)
	}}
	init.Flags().StringSliceVar(&repos, "repo", nil, "repository name=path (repeatable)")
	root.AddCommand(init)
	var dryRun, execute bool
	start := &cobra.Command{Use: "start <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, err := svc.Load(tasksRoot, args[0])
		if err != nil {
			r := report.New("start", args[0])
			r.Fail(loadDiagnostic(err))
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
	start.Flags().BoolVar(&execute, "execute", false, "execute the planned worktree actions")
	root.AddCommand(start)
	var tool string
	openCmd := &cobra.Command{Use: "open <task-id> [-- <tool-args>...]", Args: cobra.MinimumNArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, e := svc.Load(tasksRoot, args[0])
		if e != nil {
			r := report.New("open", args[0])
			r.Fail(loadDiagnostic(e))
			return render(c, r, report.ExitConfig)
		}
		r, code := svc.Open(context.Background(), t, tool, args[1:], c.InOrStdin(), c.OutOrStdout(), c.ErrOrStderr())
		return render(c, r, code)
	}}
	openCmd.Flags().StringVar(&tool, "tool", "", "codex or claude")
	root.AddCommand(openCmd)
	root.AddCommand(&cobra.Command{Use: "status <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, e := svc.Load(tasksRoot, args[0])
		if e != nil {
			r := report.New("status", args[0])
			r.Fail(loadDiagnostic(e))
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
			r.Fail(loadDiagnostic(e))
			return render(c, r, report.ExitConfig)
		}
		r, code := svc.ValidateScoped(context.Background(), t, validateRepo)
		return render(c, r, code)
	}}
	validate.Flags().StringVar(&validateRepo, "repo", "", "validate this repository and its dependencies")
	root.AddCommand(validate)
	var repoAddRepo string
	var repoAddDepends []string
	var repoAddDryRun bool
	repoCmd := &cobra.Command{Use: "repo", Short: "Manage task repositories"}
	repoAddCmd := &cobra.Command{Use: "add <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		t, e := svc.Load(tasksRoot, args[0])
		if e != nil {
			r := report.New("repo add", args[0])
			r.Fail(loadDiagnostic(e))
			return render(c, r, report.ExitConfig)
		}
		r, code := svc.RepoAdd(context.Background(), t, app.RepoAddOptions{Repository: repoAddRepo, DependsOn: repoAddDepends, DryRun: repoAddDryRun})
		return render(c, r, code)
	}}
	repoAddCmd.Flags().StringVar(&repoAddRepo, "repo", "", "repository name=absolute-path to append (required)")
	repoAddCmd.Flags().StringSliceVar(&repoAddDepends, "depends-on", nil, "existing repository the appended repo depends on (repeatable)")
	repoAddCmd.Flags().BoolVar(&repoAddDryRun, "dry-run", false, "show the resolved repository and start actions without writing")
	repoCmd.AddCommand(repoAddCmd)
	root.AddCommand(repoCmd)
	var projectSkills, forceSkills bool
	skillCmd := &cobra.Command{Use: "skill", Short: "Install Taskflow skills for AI coding agents"}
	installSkills := &cobra.Command{Use: "install", Args: cobra.NoArgs, RunE: func(c *cobra.Command, args []string) error {
		targets, err := skillTargets(projectSkills)
		r := report.New("skill install", "")
		if err != nil {
			r.Fail(report.Diagnostic{Code: "SKILL_TARGET_INVALID", Message: err.Error()})
			return render(c, r, report.ExitConfig)
		}
		installed, err := skills.Install(targets, forceSkills)
		if err != nil {
			r.Fail(report.Diagnostic{Code: "SKILL_INSTALL_FAILED", Message: err.Error()})
			return render(c, r, report.ExitExecution)
		}
		r.Data = map[string]any{"scope": skillScope(projectSkills), "targets": installed}
		return render(c, r, report.ExitOK)
	}}
	installSkills.Flags().BoolVar(&projectSkills, "project", false, "install into the current project's .codex and .claude directories")
	installSkills.Flags().BoolVar(&forceSkills, "force", false, "replace existing skills installed under the same names")
	skillCmd.AddCommand(installSkills)
	root.AddCommand(skillCmd)
	return root
}

func loadDiagnostic(err error) report.Diagnostic {
	code := "INVALID_CONFIGURATION"
	if errors.Is(err, config.ErrLegacyConfiguration) {
		code = "LEGACY_CONFIGURATION_UNSUPPORTED"
	}
	return report.Diagnostic{Code: code, Message: err.Error()}
}

func skillTargets(project bool) ([]skills.Target, error) {
	if project {
		root, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		return []skills.Target{
			{Tool: "codex", Root: filepath.Join(root, ".codex", "skills")},
			{Tool: "claude", Root: filepath.Join(root, ".claude", "skills")},
		}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" {
		codexHome = filepath.Join(home, ".codex")
	}
	return []skills.Target{
		{Tool: "codex", Root: filepath.Join(codexHome, "skills")},
		{Tool: "claude", Root: filepath.Join(home, ".claude", "skills")},
	}, nil
}

func skillScope(project bool) string {
	if project {
		return "project"
	}
	return "global"
}

type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("command failed with exit code %d", e.code) }
