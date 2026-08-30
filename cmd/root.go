package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chenquan/taskflow/internal/app"
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
	var tasksRoot = "."
	var asJSON bool
	svc := app.New()
	root := &cobra.Command{Use: "taskflow", Short: "Create and manage Git worktrees for AI coding", SilenceUsage: true}
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

	var deleteDryRun, deleteExecute, deleteForce bool
	remove := &cobra.Command{Use: "delete <task-id>", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		r, code := svc.Delete(context.Background(), app.DeleteOptions{
			TasksRoot: tasksRoot,
			TaskID:    args[0],
			DryRun:    deleteDryRun || !deleteExecute,
			Execute:   deleteExecute,
			Force:     deleteForce,
		})
		return render(c, r, code)
	}}
	remove.Flags().BoolVar(&deleteDryRun, "dry-run", false, "show the cleanup plan without changing files or Git state (default)")
	remove.Flags().BoolVar(&deleteExecute, "execute", false, "remove Taskflow-owned worktrees, local branches, and the task directory")
	remove.Flags().BoolVar(&deleteForce, "force", false, "allow deleting dirty worktrees and unmerged local branches (requires --execute)")
	root.AddCommand(remove)

	var projectSkills, forceSkills bool
	var skillTools []string
	skillCmd := &cobra.Command{Use: "skill", Short: "Install Taskflow skills for AI coding agents"}
	installSkills := &cobra.Command{Use: "install", Args: cobra.NoArgs, RunE: func(c *cobra.Command, args []string) error {
		targets, err := skillTargets(skillTools, projectSkills)
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
	installSkills.Flags().StringArrayVar(&skillTools, "tool", nil, "install for codex or claude (repeatable; defaults to both)")
	skillCmd.AddCommand(installSkills)
	root.AddCommand(skillCmd)
	return root
}

func loadDiagnostic(err error) report.Diagnostic {
	return report.Diagnostic{Code: "INVALID_CONFIGURATION", Message: err.Error()}
}

func skillTargets(selected []string, project bool) ([]skills.Target, error) {
	if len(selected) == 0 {
		selected = []string{"codex", "claude"}
	}
	tools := make([]string, 0, len(selected))
	seen := make(map[string]struct{}, len(selected))
	for _, tool := range selected {
		if tool != "codex" && tool != "claude" {
			return nil, fmt.Errorf("unsupported skill tool %q; choose codex or claude", tool)
		}
		if _, ok := seen[tool]; ok {
			continue
		}
		seen[tool] = struct{}{}
		tools = append(tools, tool)
	}

	var projectRoot, home, codexHome string
	if project {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		codexHome = os.Getenv("CODEX_HOME")
		if codexHome == "" {
			codexHome = filepath.Join(home, ".codex")
		}
	}

	targets := make([]skills.Target, 0, len(tools))
	for _, tool := range tools {
		var rootPath string
		if project {
			rootPath = filepath.Join(projectRoot, "."+tool, "skills")
		} else if tool == "codex" {
			rootPath = filepath.Join(codexHome, "skills")
		} else {
			rootPath = filepath.Join(home, ".claude", "skills")
		}
		targets = append(targets, skills.Target{Tool: tool, Root: rootPath})
	}
	return targets, nil
}

func skillScope(project bool) string {
	if project {
		return "project"
	}
	return "global"
}

type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("command failed with exit code %d", e.code) }
