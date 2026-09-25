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
	root := &cobra.Command{Use: "taskflow", Short: "Create and manage Git worktrees for AI coding", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringVar(&tasksRoot, "tasks-root", ".", "task workspace root (default: current directory)")
	root.PersistentFlags().BoolVar(&asJSON, "json", false, "emit JSON")
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		result := report.New(c.CommandPath(), "")
		result.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: err.Error()})
		if renderErr := report.Render(c.OutOrStdout(), result, asJSON); renderErr != nil {
			return renderErr
		}
		return &exitError{code: int(report.ExitConfig)}
	})
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
	create := &cobra.Command{Use: "create <task-id>", Args: exactArgs(1, &asJSON), RunE: func(c *cobra.Command, args []string) error {
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
	remove := &cobra.Command{Use: "delete <task-id>", Args: exactArgs(1, &asJSON), RunE: func(c *cobra.Command, args []string) error {
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
	installSkills := &cobra.Command{Use: "install", Args: exactArgs(0, &asJSON), RunE: func(c *cobra.Command, args []string) error {
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
	root.AddCommand(newWorkflowCommand(svc, &tasksRoot, &asJSON, render))
	return root
}

type workflowRenderer func(*cobra.Command, report.Result, report.ExitCode) error

func newWorkflowCommand(svc app.Service, tasksRoot *string, asJSON *bool, render workflowRenderer) *cobra.Command {
	workflowCmd := &cobra.Command{Use: "workflow", Short: "Run and inspect a session-driven workflow"}

	validate := &cobra.Command{Use: "validate <task-id>", Args: exactArgs(1, asJSON), RunE: func(c *cobra.Command, args []string) error {
		r, code := svc.WorkflowValidate(context.Background(), app.WorkflowOptions{TasksRoot: *tasksRoot, TaskID: args[0]})
		return render(c, r, code)
	}}
	workflowCmd.AddCommand(validate)

	status := &cobra.Command{Use: "status <task-id>", Args: exactArgs(1, asJSON), RunE: func(c *cobra.Command, args []string) error {
		r, code := svc.WorkflowStatus(context.Background(), app.WorkflowOptions{TasksRoot: *tasksRoot, TaskID: args[0]})
		return render(c, r, code)
	}}
	workflowCmd.AddCommand(status)

	var begin app.WorkflowOptions
	beginCmd := &cobra.Command{Use: "begin <task-id>", Args: exactArgs(1, asJSON), RunE: func(c *cobra.Command, args []string) error {
		begin.TasksRoot, begin.TaskID = *tasksRoot, args[0]
		r, code := svc.WorkflowBegin(context.Background(), begin)
		return render(c, r, code)
	}}
	beginCmd.Flags().StringVar(&begin.Engine, "engine", "unknown", "active host engine: codex or claude")
	beginCmd.Flags().StringVar(&begin.SessionID, "session", "", "active Agent session reference")
	beginCmd.Flags().StringVar(&begin.OperationID, "operation-id", "", "stable operation identifier for retries")
	beginCmd.Flags().DurationVar(&begin.LeaseTTL, "lease-ttl", 0, "workflow lease duration (default 15m)")
	workflowCmd.AddCommand(beginCmd)

	var checkpoint app.WorkflowOptions
	checkpointCmd := &cobra.Command{Use: "checkpoint <task-id>", Args: exactArgs(1, asJSON), RunE: func(c *cobra.Command, args []string) error {
		checkpoint.TasksRoot, checkpoint.TaskID = *tasksRoot, args[0]
		r, code := svc.WorkflowCheckpoint(context.Background(), checkpoint)
		return render(c, r, code)
	}}
	checkpointCmd.Flags().StringVar(&checkpoint.AttemptID, "attempt-id", "", "active attempt identifier")
	checkpointCmd.Flags().StringVar(&checkpoint.OwnerToken, "owner-token", "", "active workflow lease owner token")
	checkpointCmd.Flags().StringVar(&checkpoint.ReportPath, "report-file", "", "JSON checkpoint report path under the task root")
	checkpointCmd.Flags().StringVar(&checkpoint.OperationID, "operation-id", "", "stable operation identifier for retries")
	checkpointCmd.Flags().DurationVar(&checkpoint.LeaseTTL, "lease-ttl", 0, "workflow lease duration (default 15m)")
	workflowCmd.AddCommand(checkpointCmd)

	var verify app.WorkflowOptions
	verifyCmd := &cobra.Command{Use: "verify <task-id>", Args: exactArgs(1, asJSON), RunE: func(c *cobra.Command, args []string) error {
		verify.TasksRoot, verify.TaskID = *tasksRoot, args[0]
		r, code := svc.WorkflowVerify(context.Background(), verify)
		return render(c, r, code)
	}}
	verifyCmd.Flags().StringVar(&verify.AttemptID, "attempt-id", "", "active attempt identifier")
	verifyCmd.Flags().StringVar(&verify.OwnerToken, "owner-token", "", "active workflow lease owner token")
	verifyCmd.Flags().StringVar(&verify.OperationID, "operation-id", "", "stable operation identifier for retries")
	workflowCmd.AddCommand(verifyCmd)

	var pause app.WorkflowOptions
	pauseCmd := &cobra.Command{Use: "pause <task-id>", Args: exactArgs(1, asJSON), RunE: func(c *cobra.Command, args []string) error {
		pause.TasksRoot, pause.TaskID = *tasksRoot, args[0]
		r, code := svc.WorkflowPause(context.Background(), pause)
		return render(c, r, code)
	}}
	pauseCmd.Flags().StringVar(&pause.OwnerToken, "owner-token", "", "active workflow lease owner token")
	pauseCmd.Flags().StringVar(&pause.OperationID, "operation-id", "", "stable operation identifier for retries")
	pauseCmd.Flags().StringVar(&pause.Reason, "reason", "", "reason for pausing")
	workflowCmd.AddCommand(pauseCmd)

	var resume app.WorkflowOptions
	resumeCmd := &cobra.Command{Use: "resume <task-id>", Args: exactArgs(1, asJSON), RunE: func(c *cobra.Command, args []string) error {
		resume.TasksRoot, resume.TaskID = *tasksRoot, args[0]
		r, code := svc.WorkflowResume(context.Background(), resume)
		return render(c, r, code)
	}}
	resumeCmd.Flags().BoolVar(&resume.Recover, "recover", false, "explicitly recover unknown or attention state")
	resumeCmd.Flags().StringVar(&resume.OperationID, "operation-id", "", "stable operation identifier for retries")
	workflowCmd.AddCommand(resumeCmd)

	var approve app.WorkflowOptions
	approveCmd := &cobra.Command{Use: "approve <task-id> <approval-id>", Args: exactArgs(2, asJSON), RunE: func(c *cobra.Command, args []string) error {
		approve.TasksRoot, approve.TaskID, approve.ApprovalID = *tasksRoot, args[0], args[1]
		r, code := svc.WorkflowApprove(context.Background(), approve)
		return render(c, r, code)
	}}
	approveCmd.Flags().StringVar(&approve.Decision, "decision", "approve", "approval decision: approve or reject")
	approveCmd.Flags().StringVar(&approve.Reason, "reason", "", "reason for the approval decision")
	approveCmd.Flags().StringVar(&approve.OperationID, "operation-id", "", "stable operation identifier for retries")
	workflowCmd.AddCommand(approveCmd)

	var cancel app.WorkflowOptions
	cancelCmd := &cobra.Command{Use: "cancel <task-id>", Args: exactArgs(1, asJSON), RunE: func(c *cobra.Command, args []string) error {
		cancel.TasksRoot, cancel.TaskID = *tasksRoot, args[0]
		r, code := svc.WorkflowCancel(context.Background(), cancel)
		return render(c, r, code)
	}}
	cancelCmd.Flags().StringVar(&cancel.OwnerToken, "owner-token", "", "active workflow lease owner token")
	cancelCmd.Flags().StringVar(&cancel.OperationID, "operation-id", "", "stable operation identifier for retries")
	cancelCmd.Flags().StringVar(&cancel.Reason, "reason", "", "reason for cancelling")
	workflowCmd.AddCommand(cancelCmd)

	return workflowCmd
}

func exactArgs(expected int, asJSON *bool) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if len(args) == expected {
			return nil
		}
		message := fmt.Sprintf("expected %d argument(s), got %d", expected, len(args))
		result := report.New(c.CommandPath(), "")
		result.Fail(report.Diagnostic{Code: "INVALID_ARGUMENT", Message: message})
		if err := report.Render(c.OutOrStdout(), result, *asJSON); err != nil {
			return err
		}
		return &exitError{code: int(report.ExitConfig)}
	}
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
