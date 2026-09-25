package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/fsx"
	"github.com/chenquan/taskflow/internal/lock"
	"github.com/chenquan/taskflow/internal/report"
	"github.com/chenquan/taskflow/internal/workflow"
)

type WorkflowOptions struct {
	TasksRoot   string
	TaskID      string
	Engine      string
	SessionID   string
	OwnerToken  string
	AttemptID   string
	ReportPath  string
	OperationID string
	ApprovalID  string
	Decision    string
	Reason      string
	LeaseTTL    time.Duration
	Recover     bool
}

type workflowIssue struct {
	Code     string
	Repo     string
	Message  string
	Hint     string
	ExitCode report.ExitCode
}

func (e *workflowIssue) Error() string { return e.Message }

func issue(code, message string, exitCode report.ExitCode) *workflowIssue {
	return &workflowIssue{Code: code, Message: message, ExitCode: exitCode}
}

func (s Service) loadWorkflow(tasksRoot, taskID string) (domain.Task, workflow.Config, string, *workflowIssue) {
	task, err := s.Load(tasksRoot, taskID)
	if err != nil {
		return domain.Task{}, workflow.Config{}, "", issue("INVALID_CONFIGURATION", err.Error(), report.ExitConfig)
	}
	path := workflow.ConfigPath(task.Task.Root)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return domain.Task{}, workflow.Config{}, "", &workflowIssue{
			Code:     "WORKFLOW_NOT_CONFIGURED",
			Message:  fmt.Sprintf("task %s has no workflow.yaml", taskID),
			Hint:     path,
			ExitCode: report.ExitConfig,
		}
	} else if err != nil {
		return domain.Task{}, workflow.Config{}, "", issue("WORKFLOW_READ_FAILED", err.Error(), report.ExitExecution)
	}
	cfg, digest, err := workflow.Load(path, taskID)
	if err != nil {
		return domain.Task{}, workflow.Config{}, "", &workflowIssue{
			Code:     "INVALID_WORKFLOW_CONFIGURATION",
			Message:  err.Error(),
			Hint:     path,
			ExitCode: report.ExitConfig,
		}
	}
	return task, cfg, digest, nil
}

func failWorkflow(result *report.Result, problem *workflowIssue) (report.Result, report.ExitCode) {
	result.Fail(report.Diagnostic{Code: problem.Code, Repo: problem.Repo, Message: problem.Message, Hint: problem.Hint})
	return *result, problem.ExitCode
}

func (s Service) validateWorkflowWorktrees(ctx context.Context, task domain.Task, cfg workflow.Config) *workflowIssue {
	for _, repository := range task.Repositories {
		sourceInfo, err := s.Git.Inspect(ctx, repository.Source)
		if err != nil || sourceInfo.CommonDir == "" {
			return &workflowIssue{Code: "NOT_GIT_REPOSITORY", Repo: repository.Name, Message: gitErrorMessage("inspect configured source", err), ExitCode: report.ExitEnvironment}
		}
		target := filepath.Join(task.Task.Root, repository.Worktree)
		targetInfo, err := s.Git.Inspect(ctx, target)
		if err != nil {
			return &workflowIssue{Code: "WORKTREE_NOT_READY", Repo: repository.Name, Message: fmt.Sprintf("worktree %s is not ready: %v", target, err), Hint: target, ExitCode: report.ExitConflict}
		}
		if !samePath(targetInfo.CommonDir, sourceInfo.CommonDir) || targetInfo.Branch != repository.Branch {
			return &workflowIssue{
				Code:     "WORKTREE_MISMATCH",
				Repo:     repository.Name,
				Message:  fmt.Sprintf("worktree %s does not match source or branch %s", target, repository.Branch),
				Hint:     target,
				ExitCode: report.ExitConflict,
			}
		}
	}
	for _, check := range cfg.Checks {
		if _, err := workflow.ResolveCWD(task, check.CWD); err != nil {
			return &workflowIssue{Code: "INVALID_WORKFLOW_CONFIGURATION", Message: fmt.Sprintf("check %s cwd: %v", check.ID, err), ExitCode: report.ExitConfig}
		}
	}
	return nil
}

func workflowOperationID(value string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return workflow.NewID("operation")
}

func rejectOperationReuse(snapshot workflow.Snapshot, operationID, command string) *workflowIssue {
	if previous, exists := snapshot.Operations[operationID]; exists && previous.Command != command {
		return &workflowIssue{
			Code:     "OPERATION_CONFLICT",
			Message:  fmt.Sprintf("operation ID %q was already used for %s", operationID, previous.Command),
			ExitCode: report.ExitConflict,
		}
	}
	return nil
}

func (s Service) readWorkflowState(store workflow.Store, taskID string, cfg workflow.Config, digest string) (workflow.Snapshot, bool, *workflowIssue) {
	snapshot, exists, err := store.ReadSnapshot()
	if err != nil {
		return workflow.Snapshot{}, false, &workflowIssue{Code: "RUNTIME_CORRUPT", Message: err.Error(), Hint: store.Paths.State, ExitCode: report.ExitConfig}
	}
	if !exists {
		return workflow.NewSnapshot(taskID, digest, cfg, time.Now().UTC()), false, nil
	}
	if snapshot.TaskID != taskID {
		return workflow.Snapshot{}, true, &workflowIssue{Code: "RUNTIME_TASK_MISMATCH", Message: fmt.Sprintf("runtime taskID %q does not match %q", snapshot.TaskID, taskID), Hint: store.Paths.State, ExitCode: report.ExitConfig}
	}
	if snapshot.ConfigDigest == digest {
		if err := workflow.ValidateSnapshotForConfig(snapshot, cfg); err != nil {
			return workflow.Snapshot{}, true, &workflowIssue{Code: "RUNTIME_CORRUPT", Message: err.Error(), Hint: store.Paths.State, ExitCode: report.ExitConfig}
		}
	}
	return snapshot, true, nil
}

func requireWorkflowDigest(snapshot workflow.Snapshot, digest string) *workflowIssue {
	if snapshot.ConfigDigest != digest {
		return &workflowIssue{Code: "CONFIG_CHANGED", Message: "workflow.yaml changed since the active execution context was created; explicit resume is required", ExitCode: report.ExitConflict}
	}
	return nil
}

func budgetExceeded(snapshot workflow.Snapshot, cfg workflow.Config, now time.Time) (string, bool) {
	if cfg.Limits.MaxIterations > 0 && snapshot.Iteration >= cfg.Limits.MaxIterations {
		return "maximum workflow iterations exhausted", true
	}
	if cfg.Limits.MaxUsage > 0 && snapshot.Usage >= cfg.Limits.MaxUsage {
		return "maximum workflow usage exhausted", true
	}
	if cfg.Limits.MaxDuration > 0 && !snapshot.CreatedAt.IsZero() && now.Sub(snapshot.CreatedAt) >= cfg.Limits.MaxDuration.TimeDuration() {
		return "maximum workflow duration exhausted", true
	}
	return "", false
}

func workflowDurationExceeded(snapshot workflow.Snapshot, cfg workflow.Config, now time.Time) (string, bool) {
	if cfg.Limits.MaxDuration > 0 && !snapshot.CreatedAt.IsZero() && now.Sub(snapshot.CreatedAt) >= cfg.Limits.MaxDuration.TimeDuration() {
		return "maximum workflow duration exhausted", true
	}
	return "", false
}

func checkpointBudgetExceeded(snapshot workflow.Snapshot, cfg workflow.Config, status workflow.ReportStatus, now time.Time) (string, bool) {
	// A ready report belongs to the current attempt and still needs its
	// machine verification. Iteration and usage limits therefore prevent a
	// subsequent attempt, but do not discard the final verification of the
	// attempt that consumed the budget. A wall-clock deadline is different:
	// once it has elapsed, verification must not promote the attempt.
	if status == workflow.ReportReady {
		return workflowDurationExceeded(snapshot, cfg, now)
	}
	return budgetExceeded(snapshot, cfg, now)
}

func leaseTTL(value time.Duration) time.Duration {
	if value <= 0 {
		return workflow.DefaultLeaseTTL
	}
	return value
}

func validateLeaseTTL(value time.Duration) error {
	if value < 0 {
		return fmt.Errorf("lease TTL must not be negative")
	}
	return nil
}

func (s Service) readLease(store workflow.Store) (workflow.Lease, bool, *workflowIssue) {
	lease, exists, err := store.ReadLease()
	if err != nil {
		return workflow.Lease{}, false, &workflowIssue{Code: "RUNTIME_CORRUPT", Message: err.Error(), Hint: store.Paths.Lease, ExitCode: report.ExitConfig}
	}
	return lease, exists, nil
}

// recoverExpiredLocked converts an active attempt with an expired lease into
// unknown. It is intentionally called only by a mutating operation holding the
// task lock; status derives the same condition without writing files.
func recoverExpiredLocked(store workflow.Store, snapshot workflow.Snapshot, lease workflow.Lease, now time.Time) (workflow.Snapshot, bool, *workflowIssue) {
	if lease.TaskID != snapshot.TaskID {
		return snapshot, false, &workflowIssue{Code: "LEASE_CONFLICT", Message: "workflow lease belongs to another task", ExitCode: report.ExitConflict}
	}
	if lease.ExpiresAt.IsZero() || now.Before(lease.ExpiresAt) || snapshot.ActiveAttempt == nil {
		return snapshot, false, nil
	}
	if snapshot.Status != workflow.StatusRunning && snapshot.Status != workflow.StatusVerifying {
		return snapshot, false, nil
	}
	attempt := snapshot.ActiveAttempt
	attempt.Status = "unknown"
	snapshot.Status = workflow.StatusUnknown
	snapshot.LastAttemptID = attempt.ID
	snapshot.UpdatedAt = now
	event := workflow.NewEvent(snapshot.TaskID, workflow.NewID("recovery"), "attempt_unknown", snapshot, now, map[string]any{
		"reason": "workflow lease expired",
	})
	event.AttemptID = attempt.ID
	if err := store.Commit(snapshot, event, workflow.CommitOptions{ClearLease: true}); err != nil {
		return snapshot, false, &workflowIssue{Code: "RUNTIME_COMMIT_FAILED", Message: err.Error(), ExitCode: report.ExitExecution}
	}
	return snapshot, true, nil
}

func leaseMatches(lease workflow.Lease, ownerToken, taskID string, now time.Time) *workflowIssue {
	if lease.TaskID != taskID {
		return &workflowIssue{Code: "LEASE_CONFLICT", Message: "workflow lease belongs to another task", ExitCode: report.ExitConflict}
	}
	if !now.Before(lease.ExpiresAt) {
		return &workflowIssue{Code: "STALE_LEASE", Message: "workflow lease has expired; inspect the worktree and explicitly resume", ExitCode: report.ExitConflict}
	}
	if ownerToken == "" || ownerToken != lease.OwnerToken {
		return &workflowIssue{Code: "LEASE_CONFLICT", Message: "valid workflow owner token is required", ExitCode: report.ExitConflict}
	}
	return nil
}

func leaseTaskMismatch(lease workflow.Lease, exists bool, taskID string) *workflowIssue {
	if exists && lease.TaskID != taskID {
		return &workflowIssue{Code: "LEASE_CONFLICT", Message: "workflow lease belongs to another task", ExitCode: report.ExitConflict}
	}
	return nil
}

func finishAttempt(attempt *workflow.Attempt, status string, now time.Time) {
	attempt.Status = status
	attempt.FinishedAt = &now
}

func (s Service) WorkflowValidate(ctx context.Context, o WorkflowOptions) (report.Result, report.ExitCode) {
	result := report.New("workflow validate", o.TaskID)
	task, cfg, digest, problem := s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := s.validateWorkflowWorktrees(ctx, task, cfg); problem != nil {
		return failWorkflow(&result, problem)
	}
	result.Data = map[string]any{
		"taskRoot":      task.Task.Root,
		"workflowPath":  workflow.ConfigPath(task.Task.Root),
		"configDigest":  digest,
		"configuration": cfg,
	}
	return result, report.ExitOK
}

func (s Service) WorkflowStatus(ctx context.Context, o WorkflowOptions) (report.Result, report.ExitCode) {
	result := report.New("workflow status", o.TaskID)
	task, cfg, digest, problem := s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	store := workflow.NewStore(task.Task.Root)
	snapshot, exists, problem := s.readWorkflowState(store, task.Task.ID, cfg, digest)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	lease, leaseExists, problem := s.readLease(store)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	leaseExpired := leaseExists && !time.Now().UTC().Before(lease.ExpiresAt)
	if leaseExpired && snapshot.ActiveAttempt != nil && (snapshot.Status == workflow.StatusRunning || snapshot.Status == workflow.StatusVerifying) {
		snapshot.Status = workflow.StatusUnknown
		snapshot.ActiveAttempt.Status = "unknown"
		result.Warn(report.Diagnostic{Code: "STALE_LEASE", Message: "active lease expired; workflow requires explicit recovery before continuing"})
	}
	if leaseExists && lease.TaskID != task.Task.ID {
		result.Warn(report.Diagnostic{Code: "LEASE_MISMATCH", Message: "workflow lease belongs to another task", Hint: store.Paths.Lease})
		if !snapshot.IsTerminal() {
			snapshot.Status = workflow.StatusUnknown
			if snapshot.ActiveAttempt != nil {
				snapshot.ActiveAttempt.Status = "unknown"
			}
		}
	}
	if snapshot.ActiveAttempt != nil && (snapshot.Status == workflow.StatusRunning || snapshot.Status == workflow.StatusVerifying) && !leaseExists {
		result.Warn(report.Diagnostic{Code: "LEASE_MISSING", Message: "active workflow attempt has no lease; explicit recovery is required", Hint: store.Paths.Lease})
		snapshot.Status = workflow.StatusUnknown
		snapshot.ActiveAttempt.Status = "unknown"
	}
	if snapshot.ActiveAttempt == nil && (snapshot.Status == workflow.StatusRunning || snapshot.Status == workflow.StatusVerifying) {
		result.Warn(report.Diagnostic{Code: "ACTIVE_ATTEMPT_MISSING", Message: "workflow state requires an active attempt but none exists", Hint: store.Paths.State})
		snapshot.Status = workflow.StatusUnknown
	}
	if leaseExists && snapshot.ActiveAttempt == nil && !snapshot.IsTerminal() && snapshot.Status != workflow.StatusAwaitingApproval {
		result.Warn(report.Diagnostic{Code: "ORPHAN_LEASE", Message: "workflow lease exists without an active attempt; explicit recovery is required", Hint: store.Paths.Lease})
		snapshot.Status = workflow.StatusUnknown
	}
	if exists && snapshot.ConfigDigest != digest && !snapshot.IsTerminal() {
		result.Warn(report.Diagnostic{Code: "CONFIG_CHANGED", Message: "workflow.yaml digest differs from the active execution context"})
		snapshot.Status = workflow.StatusUnknown
		if snapshot.ActiveAttempt != nil {
			snapshot.ActiveAttempt.Status = "unknown"
		}
	}
	if preflight := s.validateWorkflowWorktrees(ctx, task, cfg); preflight != nil {
		result.Warn(report.Diagnostic{Code: preflight.Code, Repo: preflight.Repo, Message: preflight.Message, Hint: preflight.Hint})
	}
	events, err := store.ReadEvents(20)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_CORRUPT", Message: err.Error(), Hint: store.Paths.Events, ExitCode: report.ExitConfig})
	}
	result.Data = map[string]any{
		"configured":     true,
		"taskRoot":       task.Task.Root,
		"workflowPath":   workflow.ConfigPath(task.Task.Root),
		"configDigest":   digest,
		"status":         snapshot.Status,
		"stage":          snapshot.StageID,
		"iteration":      snapshot.Iteration,
		"snapshotExists": exists,
		"snapshot":       snapshot,
		"lease":          lease,
		"leaseExists":    leaseExists,
		"leaseExpired":   leaseExpired,
		"events":         events,
	}
	return result, report.ExitOK
}

func (s Service) WorkflowBegin(ctx context.Context, o WorkflowOptions) (report.Result, report.ExitCode) {
	result := report.New("workflow begin", o.TaskID)
	engine, err := workflowEngine(o.Engine)
	if err != nil {
		return failWorkflow(&result, issue("INVALID_ARGUMENT", err.Error(), report.ExitConfig))
	}
	if err := validateLeaseTTL(o.LeaseTTL); err != nil {
		return failWorkflow(&result, issue("INVALID_ARGUMENT", err.Error(), report.ExitConfig))
	}
	task, cfg, digest, problem := s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := s.validateWorkflowWorktrees(ctx, task, cfg); problem != nil {
		return failWorkflow(&result, problem)
	}
	taskLock, err := lock.Acquire(task.Task.Root)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "TASK_LOCKED", Message: err.Error(), ExitCode: report.ExitConflict})
	}
	defer taskLock.Release()
	task, cfg, digest, problem = s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := s.validateWorkflowWorktrees(ctx, task, cfg); problem != nil {
		return failWorkflow(&result, problem)
	}
	store := workflow.NewStore(task.Task.Root)
	snapshot, _, problem := s.readWorkflowState(store, task.Task.ID, cfg, digest)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	opID := workflowOperationID(o.OperationID)
	if previous, ok := snapshot.Operation(opID, "workflow begin"); ok {
		result.Data = previous.Result
		return result, report.ExitOK
	}
	if problem := rejectOperationReuse(snapshot, opID, "workflow begin"); problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := requireWorkflowDigest(snapshot, digest); problem != nil {
		return failWorkflow(&result, problem)
	}
	lease, leaseExists, problem := s.readLease(store)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	now := time.Now().UTC()
	if leaseExists {
		var recovered bool
		snapshot, recovered, problem = recoverExpiredLocked(store, snapshot, lease, now)
		if problem != nil {
			return failWorkflow(&result, problem)
		}
		if recovered {
			return failWorkflow(&result, &workflowIssue{Code: "STALE_LEASE", Message: "previous workflow lease expired; inspect the worktree before explicitly resuming", ExitCode: report.ExitConflict})
		}
		return failWorkflow(&result, &workflowIssue{Code: "LEASE_CONFLICT", Message: "another workflow session owns this task", ExitCode: report.ExitConflict})
	}
	if snapshot.PendingApproval != nil {
		return failWorkflow(&result, &workflowIssue{Code: "APPROVAL_REQUIRED", Message: "resolve the pending approval before beginning another attempt", ExitCode: report.ExitConflict})
	}
	if snapshot.Status != workflow.StatusReady {
		return failWorkflow(&result, &workflowIssue{Code: "WORKFLOW_NOT_READY", Message: fmt.Sprintf("workflow is in %s; use resume or resolve the current attention state", snapshot.Status), ExitCode: report.ExitConflict})
	}
	if reason, exceeded := budgetExceeded(snapshot, cfg, now); exceeded {
		return failWorkflow(&result, &workflowIssue{Code: "BUDGET_EXHAUSTED", Message: reason, ExitCode: report.ExitConflict})
	}
	stage, ok := workflow.StageAt(cfg, snapshot.StageIndex)
	if !ok || stage.ID != snapshot.StageID {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_STAGE_INVALID", Message: "runtime stage does not match workflow configuration", Hint: store.Paths.State, ExitCode: report.ExitConfig})
	}
	if snapshot.StageAttempts[stage.ID] >= stage.MaxAttempts {
		return failWorkflow(&result, &workflowIssue{Code: "ATTEMPT_LIMIT_EXHAUSTED", Message: fmt.Sprintf("stage %s attempt limit is exhausted", stage.ID), ExitCode: report.ExitConflict})
	}
	attemptID := workflow.NewID("attempt")
	ownerToken := workflow.NewID("owner")
	attemptNumber := snapshot.StageAttempts[stage.ID] + 1
	attempt := &workflow.Attempt{
		ID:           attemptID,
		StageID:      stage.ID,
		Iteration:    snapshot.Iteration + 1,
		StageAttempt: attemptNumber,
		SessionID:    o.SessionID,
		Status:       "active",
		StartedAt:    now,
	}
	paths, err := store.Paths.Attempt(attemptID)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "INVALID_ATTEMPT", Message: err.Error(), ExitCode: report.ExitConfig})
	}
	attempt.ReportPath, _ = filepath.Rel(task.Task.Root, paths.Report)
	if err := store.SavePrompt(attemptID, fmt.Sprintf("# Workflow attempt\n\nTask: %s\nStage: %s\n\n%s\n", task.Task.ID, stage.ID, stage.Objective)); err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_EVIDENCE_FAILED", Message: err.Error(), ExitCode: report.ExitExecution})
	}
	snapshot.Status = workflow.StatusRunning
	snapshot.ActiveAttempt = attempt
	snapshot.LastAttemptID = attemptID
	snapshot.Iteration++
	snapshot.StageAttempts[stage.ID] = attemptNumber
	snapshot.UpdatedAt = now
	lease = workflow.Lease{
		Version:    workflow.RuntimeVersion,
		TaskID:     task.Task.ID,
		Engine:     engine,
		SessionID:  o.SessionID,
		OwnerToken: ownerToken,
		CreatedAt:  now,
		ExpiresAt:  now.Add(leaseTTL(o.LeaseTTL)),
	}
	data := map[string]any{
		"operationID":    opID,
		"attemptID":      attemptID,
		"stageID":        stage.ID,
		"objective":      stage.Objective,
		"iteration":      snapshot.Iteration,
		"stageAttempt":   attemptNumber,
		"ownerToken":     ownerToken,
		"leaseExpiresAt": lease.ExpiresAt,
		"reportPath":     attempt.ReportPath,
	}
	snapshot.RecordOperation(opID, "workflow begin", data, now)
	event := workflow.NewEvent(task.Task.ID, opID, "begin", snapshot, now, map[string]any{
		"engine":    lease.Engine,
		"sessionID": o.SessionID,
		"objective": stage.Objective,
	})
	if err := store.Commit(snapshot, event, workflow.CommitOptions{Lease: &lease}); err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_COMMIT_FAILED", Message: err.Error(), ExitCode: report.ExitExecution})
	}
	result.Data = data
	return result, report.ExitOK
}

func (s Service) WorkflowCheckpoint(ctx context.Context, o WorkflowOptions) (report.Result, report.ExitCode) {
	result := report.New("workflow checkpoint", o.TaskID)
	if err := validateLeaseTTL(o.LeaseTTL); err != nil {
		return failWorkflow(&result, issue("INVALID_ARGUMENT", err.Error(), report.ExitConfig))
	}
	task, cfg, digest, problem := s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if strings.TrimSpace(o.ReportPath) == "" {
		return failWorkflow(&result, issue("INVALID_ARGUMENT", "--report-file is required", report.ExitConfig))
	}
	taskLock, err := lock.Acquire(task.Task.Root)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "TASK_LOCKED", Message: err.Error(), ExitCode: report.ExitConflict})
	}
	defer taskLock.Release()
	task, cfg, digest, problem = s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := s.validateWorkflowWorktrees(ctx, task, cfg); problem != nil {
		return failWorkflow(&result, problem)
	}
	store := workflow.NewStore(task.Task.Root)
	snapshot, exists, problem := s.readWorkflowState(store, task.Task.ID, cfg, digest)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if !exists {
		return failWorkflow(&result, issue("WORKFLOW_NOT_STARTED", "workflow has no active attempt; run workflow begin first", report.ExitConflict))
	}
	opID := workflowOperationID(o.OperationID)
	if previous, ok := snapshot.Operation(opID, "workflow checkpoint"); ok {
		result.Data = previous.Result
		return result, report.ExitOK
	}
	if problem := rejectOperationReuse(snapshot, opID, "workflow checkpoint"); problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := requireWorkflowDigest(snapshot, digest); problem != nil {
		return failWorkflow(&result, problem)
	}
	if snapshot.ActiveAttempt == nil || snapshot.ActiveAttempt.ID != o.AttemptID {
		return failWorkflow(&result, &workflowIssue{Code: "ATTEMPT_CONFLICT", Message: "checkpoint does not reference the active attempt", ExitCode: report.ExitConflict})
	}
	lease, leaseExists, problem := s.readLease(store)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if !leaseExists {
		return failWorkflow(&result, &workflowIssue{Code: "LEASE_CONFLICT", Message: "active workflow lease is missing", ExitCode: report.ExitConflict})
	}
	now := time.Now().UTC()
	snapshot, recovered, problem := recoverExpiredLocked(store, snapshot, lease, now)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if recovered {
		return failWorkflow(&result, &workflowIssue{Code: "STALE_LEASE", Message: "workflow lease expired; checkpoint was not accepted", ExitCode: report.ExitConflict})
	}
	if problem := leaseMatches(lease, o.OwnerToken, task.Task.ID, now); problem != nil {
		return failWorkflow(&result, problem)
	}
	if snapshot.Status != workflow.StatusRunning {
		return failWorkflow(&result, &workflowIssue{Code: "CHECKPOINT_NOT_ALLOWED", Message: fmt.Sprintf("checkpoint is not allowed from workflow state %s", snapshot.Status), ExitCode: report.ExitConflict})
	}
	reportRaw, err := readReportFile(task.Task.Root, o.ReportPath)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "INVALID_CHECKPOINT", Message: err.Error(), ExitCode: report.ExitConfig})
	}
	agentReport, err := workflow.DecodeReport(reportRaw)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "INVALID_CHECKPOINT", Message: err.Error(), ExitCode: report.ExitConfig})
	}
	if err := workflow.ValidateReport(agentReport); err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "INVALID_CHECKPOINT", Message: err.Error(), ExitCode: report.ExitConfig})
	}
	active := snapshot.ActiveAttempt
	if agentReport.TaskID != task.Task.ID || agentReport.StageID != active.StageID || agentReport.AttemptID != active.ID {
		return failWorkflow(&result, &workflowIssue{Code: "CHECKPOINT_IDENTITY_MISMATCH", Message: "checkpoint task, stage, or attempt does not match the active workflow", ExitCode: report.ExitConflict})
	}
	if agentReport.SessionID != "" && active.SessionID != "" && agentReport.SessionID != active.SessionID {
		return failWorkflow(&result, &workflowIssue{Code: "CHECKPOINT_SESSION_MISMATCH", Message: "checkpoint session does not match the active workflow lease", ExitCode: report.ExitConflict})
	}
	if agentReport.Status == workflow.ReportNeedsApproval {
		if cfg.Policy.ExternalActions != "approval" {
			return failWorkflow(&result, &workflowIssue{Code: "ACTION_POLICY_DENIED", Message: "workflow policy does not allow approval-gated external actions", ExitCode: report.ExitConflict})
		}
		if len(cfg.Policy.AllowedActions) > 0 && !containsAction(cfg.Policy.AllowedActions, agentReport.Approval.Action) {
			return failWorkflow(&result, &workflowIssue{Code: "ACTION_NOT_ALLOWED", Message: fmt.Sprintf("approval action %q is not listed in workflow policy", agentReport.Approval.Action), ExitCode: report.ExitConflict})
		}
		for _, previous := range snapshot.Approvals {
			if previous.ID == agentReport.Approval.ID {
				return failWorkflow(&result, &workflowIssue{Code: "APPROVAL_CONFLICT", Message: fmt.Sprintf("approval ID %q has already been recorded", agentReport.Approval.ID), ExitCode: report.ExitConflict})
			}
		}
	}
	reportDigest, err := workflow.ReportDigest(agentReport)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "INVALID_CHECKPOINT", Message: err.Error(), ExitCode: report.ExitConfig})
	}
	if err := store.SaveReport(active.ID, agentReport); err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_EVIDENCE_FAILED", Message: err.Error(), ExitCode: report.ExitExecution})
	}
	activePaths, err := store.Paths.Attempt(active.ID)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "INVALID_ATTEMPT", Message: err.Error(), ExitCode: report.ExitConfig})
	}
	active.ReportPath, _ = filepath.Rel(task.Task.Root, activePaths.Report)
	active.ReportDigest = reportDigest
	snapshot.Usage += agentReport.Usage
	snapshot.LastAttemptID = active.ID
	nextLease := lease
	nextLease.ExpiresAt = now.Add(leaseTTL(o.LeaseTTL))
	clearLease := false
	switch agentReport.Status {
	case workflow.ReportProgress:
		snapshot.Status = workflow.StatusRunning
	case workflow.ReportReady:
		snapshot.Status = workflow.StatusVerifying
	case workflow.ReportBlocked:
		finishAttempt(active, "blocked", now)
		snapshot.ActiveAttempt = nil
		snapshot.Status = workflow.StatusNeedsAttention
		clearLease = true
	case workflow.ReportNeedsApproval:
		approval := workflow.Approval{
			ID:          agentReport.Approval.ID,
			Action:      agentReport.Approval.Action,
			Description: agentReport.Approval.Description,
			RequestedAt: now,
		}
		snapshot.Approvals = append(snapshot.Approvals, approval)
		snapshot.PendingApproval = &approval
		finishAttempt(active, "awaiting_approval", now)
		snapshot.ActiveAttempt = nil
		snapshot.Status = workflow.StatusAwaitingApproval
		clearLease = true
	}
	if reason, exceeded := checkpointBudgetExceeded(snapshot, cfg, agentReport.Status, now); exceeded && (snapshot.Status == workflow.StatusRunning || snapshot.Status == workflow.StatusVerifying) {
		finishAttempt(active, "budget_exhausted", now)
		snapshot.ActiveAttempt = nil
		snapshot.Status = workflow.StatusNeedsAttention
		clearLease = true
		result.Warn(report.Diagnostic{Code: "BUDGET_EXHAUSTED", Message: reason})
	}
	snapshot.UpdatedAt = now
	data := map[string]any{
		"operationID":  opID,
		"attemptID":    active.ID,
		"stageID":      active.StageID,
		"reportStatus": agentReport.Status,
		"status":       snapshot.Status,
		"reportPath":   active.ReportPath,
	}
	snapshot.RecordOperation(opID, "workflow checkpoint", data, now)
	event := workflow.NewEvent(task.Task.ID, opID, "checkpoint", snapshot, now, map[string]any{
		"reportStatus": agentReport.Status,
		"summary":      agentReport.Summary,
	})
	event.AttemptID = active.ID
	options := workflow.CommitOptions{Lease: &nextLease}
	if clearLease {
		options = workflow.CommitOptions{ClearLease: true}
	}
	if err := store.Commit(snapshot, event, options); err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_COMMIT_FAILED", Message: err.Error(), ExitCode: report.ExitExecution})
	}
	result.Data = data
	return result, report.ExitOK
}

func (s Service) WorkflowVerify(ctx context.Context, o WorkflowOptions) (report.Result, report.ExitCode) {
	result := report.New("workflow verify", o.TaskID)
	task, cfg, digest, problem := s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	taskLock, err := lock.Acquire(task.Task.Root)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "TASK_LOCKED", Message: err.Error(), ExitCode: report.ExitConflict})
	}
	defer taskLock.Release()
	task, cfg, digest, problem = s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := s.validateWorkflowWorktrees(ctx, task, cfg); problem != nil {
		return failWorkflow(&result, problem)
	}
	store := workflow.NewStore(task.Task.Root)
	snapshot, exists, problem := s.readWorkflowState(store, task.Task.ID, cfg, digest)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if !exists {
		return failWorkflow(&result, issue("WORKFLOW_NOT_STARTED", "workflow has no active attempt", report.ExitConflict))
	}
	opID := workflowOperationID(o.OperationID)
	if previous, ok := snapshot.Operation(opID, "workflow verify"); ok {
		result.Data = previous.Result
		return result, report.ExitOK
	}
	if problem := rejectOperationReuse(snapshot, opID, "workflow verify"); problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := requireWorkflowDigest(snapshot, digest); problem != nil {
		return failWorkflow(&result, problem)
	}
	if snapshot.ActiveAttempt == nil {
		return failWorkflow(&result, issue("WORKFLOW_NOT_STARTED", "workflow has no active attempt", report.ExitConflict))
	}
	if o.AttemptID != "" && snapshot.ActiveAttempt.ID != o.AttemptID {
		return failWorkflow(&result, &workflowIssue{Code: "ATTEMPT_CONFLICT", Message: "verify does not reference the active attempt", ExitCode: report.ExitConflict})
	}
	lease, leaseExists, problem := s.readLease(store)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if !leaseExists {
		return failWorkflow(&result, &workflowIssue{Code: "LEASE_CONFLICT", Message: "active workflow lease is missing", ExitCode: report.ExitConflict})
	}
	now := time.Now().UTC()
	snapshot, recovered, problem := recoverExpiredLocked(store, snapshot, lease, now)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if recovered {
		return failWorkflow(&result, &workflowIssue{Code: "STALE_LEASE", Message: "workflow lease expired; verification was not run", ExitCode: report.ExitConflict})
	}
	if problem := leaseMatches(lease, o.OwnerToken, task.Task.ID, now); problem != nil {
		return failWorkflow(&result, problem)
	}
	if snapshot.Status != workflow.StatusVerifying {
		return failWorkflow(&result, &workflowIssue{Code: "VERIFY_NOT_ALLOWED", Message: fmt.Sprintf("verify is not allowed from workflow state %s", snapshot.Status), ExitCode: report.ExitConflict})
	}
	active := snapshot.ActiveAttempt
	agentReport, exists, err := store.ReadReport(active.ID)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_CORRUPT", Message: err.Error(), ExitCode: report.ExitConfig})
	}
	if !exists || agentReport.Status != workflow.ReportReady {
		return failWorkflow(&result, &workflowIssue{Code: "CHECKPOINT_REQUIRED", Message: "a ready checkpoint is required before verification", ExitCode: report.ExitConflict})
	}
	if err := workflow.ValidateReport(agentReport); err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_CORRUPT", Message: fmt.Sprintf("stored checkpoint report is invalid: %v", err), Hint: active.ReportPath, ExitCode: report.ExitConfig})
	}
	if agentReport.TaskID != task.Task.ID || agentReport.StageID != active.StageID || agentReport.AttemptID != active.ID {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_CORRUPT", Message: "stored checkpoint report identity does not match the active workflow", Hint: active.ReportPath, ExitCode: report.ExitConfig})
	}
	if agentReport.SessionID != "" && active.SessionID != "" && agentReport.SessionID != active.SessionID {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_CORRUPT", Message: "stored checkpoint report session does not match the active workflow", Hint: active.ReportPath, ExitCode: report.ExitConfig})
	}
	if active.ReportDigest == "" {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_CORRUPT", Message: "active attempt has no checkpoint report digest", Hint: active.ReportPath, ExitCode: report.ExitConfig})
	}
	storedReportDigest, err := workflow.ReportDigest(agentReport)
	if err != nil || storedReportDigest != active.ReportDigest {
		message := "stored checkpoint report digest does not match the checkpoint evidence"
		if err != nil {
			message = err.Error()
		}
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_CORRUPT", Message: message, Hint: active.ReportPath, ExitCode: report.ExitConfig})
	}
	stage, ok := workflow.StageAt(cfg, snapshot.StageIndex)
	if !ok || stage.ID != snapshot.StageID {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_STAGE_INVALID", Message: "runtime stage does not match workflow configuration", ExitCode: report.ExitConfig})
	}
	verification, err := workflow.RunChecks(ctx, task, cfg, stage, s.Runner)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "CHECK_EXECUTION_INVALID", Message: err.Error(), ExitCode: report.ExitConfig})
	}
	verification.AttemptID = active.ID
	failed := make([]string, 0)
	checkIDs := make([]string, 0, len(verification.Checks))
	for index := range verification.Checks {
		verification.Checks[index].AttemptID = active.ID
		checkIDs = append(checkIDs, verification.Checks[index].ID)
		if !verification.Checks[index].Passed {
			failed = append(failed, verification.Checks[index].ID)
		}
		if err := store.SaveCheckResult(verification.Checks[index]); err != nil {
			return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_EVIDENCE_FAILED", Message: err.Error(), ExitCode: report.ExitExecution})
		}
	}
	finishedAt := time.Now().UTC()
	snapshot.LastVerification = &workflow.VerificationSummary{Passed: verification.Passed, CheckIDs: checkIDs, FailedCheck: failed, CompletedAt: verification.CompletedAt}
	snapshot.LastAttemptID = active.ID
	durationReason, durationExceeded := workflowDurationExceeded(snapshot, cfg, finishedAt)
	attemptStatus := map[bool]string{true: "verified", false: "failed"}[verification.Passed]
	if durationExceeded {
		attemptStatus = "budget_exhausted"
	}
	finishAttempt(active, attemptStatus, finishedAt)
	snapshot.ActiveAttempt = nil
	if durationExceeded {
		snapshot.Status = workflow.StatusNeedsAttention
		result.Warn(report.Diagnostic{Code: "BUDGET_EXHAUSTED", Message: durationReason})
	} else if verification.Passed {
		if snapshot.StageIndex+1 >= len(cfg.Stages) {
			snapshot.Status = workflow.StatusCompleted
		} else {
			snapshot.StageIndex++
			snapshot.StageID = cfg.Stages[snapshot.StageIndex].ID
			snapshot.Status = workflow.StatusReady
		}
	} else {
		reason, exceeded := budgetExceeded(snapshot, cfg, finishedAt)
		if exceeded || snapshot.StageAttempts[stage.ID] >= stage.MaxAttempts {
			snapshot.Status = workflow.StatusNeedsAttention
			if reason == "" {
				reason = fmt.Sprintf("stage %s attempt limit is exhausted", stage.ID)
			}
			result.Warn(report.Diagnostic{Code: "NEEDS_ATTENTION", Message: reason})
		} else {
			snapshot.Status = workflow.StatusReady
		}
	}
	snapshot.UpdatedAt = finishedAt
	data := map[string]any{
		"operationID":  opID,
		"attemptID":    active.ID,
		"stageID":      stage.ID,
		"passed":       verification.Passed,
		"failedChecks": failed,
		"status":       snapshot.Status,
		"checks":       verification.Checks,
	}
	snapshot.RecordOperation(opID, "workflow verify", map[string]any{
		"operationID":  opID,
		"attemptID":    active.ID,
		"stageID":      stage.ID,
		"passed":       verification.Passed,
		"failedChecks": failed,
		"status":       snapshot.Status,
	}, finishedAt)
	event := workflow.NewEvent(task.Task.ID, opID, "verify", snapshot, finishedAt, map[string]any{
		"passed":       verification.Passed,
		"failedChecks": failed,
	})
	event.AttemptID = active.ID
	if err := store.Commit(snapshot, event, workflow.CommitOptions{ClearLease: true}); err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_COMMIT_FAILED", Message: err.Error(), ExitCode: report.ExitExecution})
	}
	result.Data = data
	return result, report.ExitOK
}

func (s Service) WorkflowPause(ctx context.Context, o WorkflowOptions) (report.Result, report.ExitCode) {
	result := report.New("workflow pause", o.TaskID)
	task, cfg, digest, problem := s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	taskLock, err := lock.Acquire(task.Task.Root)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "TASK_LOCKED", Message: err.Error(), ExitCode: report.ExitConflict})
	}
	defer taskLock.Release()
	task, cfg, digest, problem = s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	store := workflow.NewStore(task.Task.Root)
	snapshot, exists, problem := s.readWorkflowState(store, task.Task.ID, cfg, digest)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if !exists {
		return failWorkflow(&result, issue("WORKFLOW_NOT_STARTED", "workflow has not started", report.ExitConflict))
	}
	opID := workflowOperationID(o.OperationID)
	if previous, ok := snapshot.Operation(opID, "workflow pause"); ok {
		result.Data = previous.Result
		return result, report.ExitOK
	}
	if problem := rejectOperationReuse(snapshot, opID, "workflow pause"); problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := requireWorkflowDigest(snapshot, digest); problem != nil {
		return failWorkflow(&result, problem)
	}
	lease, leaseExists, problem := s.readLease(store)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := leaseTaskMismatch(lease, leaseExists, task.Task.ID); problem != nil {
		return failWorkflow(&result, problem)
	}
	if snapshot.IsTerminal() {
		return failWorkflow(&result, &workflowIssue{Code: "WORKFLOW_TERMINAL", Message: fmt.Sprintf("workflow is already %s", snapshot.Status), ExitCode: report.ExitConflict})
	}
	if snapshot.Status == workflow.StatusAwaitingApproval || snapshot.PendingApproval != nil {
		return failWorkflow(&result, &workflowIssue{Code: "APPROVAL_REQUIRED", Message: "resolve the pending approval before pausing the workflow", ExitCode: report.ExitConflict})
	}
	now := time.Now().UTC()
	if snapshot.ActiveAttempt != nil {
		if !leaseExists {
			return failWorkflow(&result, &workflowIssue{Code: "LEASE_CONFLICT", Message: "active workflow lease is missing; recover before pausing the attempt", ExitCode: report.ExitConflict})
		}
		if problem := leaseMatches(lease, o.OwnerToken, task.Task.ID, now); problem != nil {
			return failWorkflow(&result, problem)
		}
		finishAttempt(snapshot.ActiveAttempt, "paused", now)
		snapshot.LastAttemptID = snapshot.ActiveAttempt.ID
		snapshot.ActiveAttempt = nil
	}
	snapshot.Status = workflow.StatusPaused
	snapshot.UpdatedAt = now
	data := map[string]any{"operationID": opID, "status": snapshot.Status, "reason": o.Reason}
	snapshot.RecordOperation(opID, "workflow pause", data, now)
	event := workflow.NewEvent(task.Task.ID, opID, "pause", snapshot, now, map[string]any{"reason": o.Reason})
	if err := store.Commit(snapshot, event, workflow.CommitOptions{ClearLease: true}); err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_COMMIT_FAILED", Message: err.Error(), ExitCode: report.ExitExecution})
	}
	result.Data = data
	return result, report.ExitOK
}

func (s Service) WorkflowResume(ctx context.Context, o WorkflowOptions) (report.Result, report.ExitCode) {
	result := report.New("workflow resume", o.TaskID)
	task, cfg, digest, problem := s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	taskLock, err := lock.Acquire(task.Task.Root)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "TASK_LOCKED", Message: err.Error(), ExitCode: report.ExitConflict})
	}
	defer taskLock.Release()
	task, cfg, digest, problem = s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := s.validateWorkflowWorktrees(ctx, task, cfg); problem != nil {
		return failWorkflow(&result, problem)
	}
	store := workflow.NewStore(task.Task.Root)
	snapshot, exists, problem := s.readWorkflowState(store, task.Task.ID, cfg, digest)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if !exists {
		return failWorkflow(&result, issue("WORKFLOW_NOT_STARTED", "workflow has not started", report.ExitConflict))
	}
	opID := workflowOperationID(o.OperationID)
	if previous, ok := snapshot.Operation(opID, "workflow resume"); ok {
		result.Data = previous.Result
		return result, report.ExitOK
	}
	if problem := rejectOperationReuse(snapshot, opID, "workflow resume"); problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := requireWorkflowDigest(snapshot, digest); problem != nil {
		if !o.Recover {
			return failWorkflow(&result, problem)
		}
		snapshot.ConfigDigest = digest
		snapshot.StageAttempts = map[string]int{}
		snapshot.Iteration = 0
		snapshot.Usage = 0
		snapshot.StageIndex = 0
		snapshot.StageID = cfg.Stages[0].ID
		snapshot.LastVerification = nil
	}
	lease, leaseExists, problem := s.readLease(store)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := leaseTaskMismatch(lease, leaseExists, task.Task.ID); problem != nil {
		return failWorkflow(&result, problem)
	}
	if snapshot.Status == workflow.StatusRunning || snapshot.Status == workflow.StatusVerifying {
		if leaseExists && !time.Now().UTC().Before(lease.ExpiresAt) {
			snapshot, _, problem = recoverExpiredLocked(store, snapshot, lease, time.Now().UTC())
			if problem != nil {
				return failWorkflow(&result, problem)
			}
		} else {
			return failWorkflow(&result, &workflowIssue{Code: "LEASE_CONFLICT", Message: "active workflow session still owns this task", ExitCode: report.ExitConflict})
		}
	}
	now := time.Now().UTC()
	switch snapshot.Status {
	case workflow.StatusPaused:
		if snapshot.PendingApproval != nil {
			return failWorkflow(&result, &workflowIssue{Code: "APPROVAL_REQUIRED", Message: "resolve the pending approval before resuming the workflow", ExitCode: report.ExitConflict})
		}
		snapshot.Status = workflow.StatusReady
	case workflow.StatusUnknown, workflow.StatusNeedsAttention:
		if !o.Recover {
			return failWorkflow(&result, &workflowIssue{Code: "RECOVERY_REQUIRED", Message: "explicit --recover is required for unknown or needs_attention workflow state", ExitCode: report.ExitConflict})
		}
		if snapshot.ActiveAttempt != nil {
			finishAttempt(snapshot.ActiveAttempt, "recovered", now)
			snapshot.LastAttemptID = snapshot.ActiveAttempt.ID
			snapshot.ActiveAttempt = nil
		}
		snapshot.Status = workflow.StatusReady
	case workflow.StatusReady:
		// Treat resume on an already-ready workflow as an idempotent no-op.
	case workflow.StatusAwaitingApproval:
		return failWorkflow(&result, &workflowIssue{Code: "APPROVAL_REQUIRED", Message: "resolve the pending approval before resuming", ExitCode: report.ExitConflict})
	case workflow.StatusCompleted, workflow.StatusCancelled:
		return failWorkflow(&result, &workflowIssue{Code: "WORKFLOW_TERMINAL", Message: fmt.Sprintf("workflow is already %s", snapshot.Status), ExitCode: report.ExitConflict})
	default:
		return failWorkflow(&result, &workflowIssue{Code: "WORKFLOW_STATE_INVALID", Message: fmt.Sprintf("cannot resume workflow from %s", snapshot.Status), ExitCode: report.ExitConfig})
	}
	snapshot.UpdatedAt = now
	data := map[string]any{"operationID": opID, "status": snapshot.Status, "recovered": o.Recover}
	snapshot.RecordOperation(opID, "workflow resume", data, now)
	event := workflow.NewEvent(task.Task.ID, opID, "resume", snapshot, now, map[string]any{"recovered": o.Recover})
	if err := store.Commit(snapshot, event, workflow.CommitOptions{ClearLease: true}); err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_COMMIT_FAILED", Message: err.Error(), ExitCode: report.ExitExecution})
	}
	result.Data = data
	return result, report.ExitOK
}

func (s Service) WorkflowApprove(ctx context.Context, o WorkflowOptions) (report.Result, report.ExitCode) {
	result := report.New("workflow approve", o.TaskID)
	task, cfg, digest, problem := s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	taskLock, err := lock.Acquire(task.Task.Root)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "TASK_LOCKED", Message: err.Error(), ExitCode: report.ExitConflict})
	}
	defer taskLock.Release()
	task, cfg, digest, problem = s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	store := workflow.NewStore(task.Task.Root)
	snapshot, exists, problem := s.readWorkflowState(store, task.Task.ID, cfg, digest)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if !exists {
		return failWorkflow(&result, issue("WORKFLOW_NOT_STARTED", "workflow has not started", report.ExitConflict))
	}
	opID := workflowOperationID(o.OperationID)
	if previous, ok := snapshot.Operation(opID, "workflow approve"); ok {
		result.Data = previous.Result
		return result, report.ExitOK
	}
	if problem := rejectOperationReuse(snapshot, opID, "workflow approve"); problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := requireWorkflowDigest(snapshot, digest); problem != nil {
		return failWorkflow(&result, problem)
	}
	lease, leaseExists, problem := s.readLease(store)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := leaseTaskMismatch(lease, leaseExists, task.Task.ID); problem != nil {
		return failWorkflow(&result, problem)
	}
	if snapshot.Status != workflow.StatusAwaitingApproval || snapshot.PendingApproval == nil {
		return failWorkflow(&result, &workflowIssue{Code: "APPROVAL_CONFLICT", Message: "workflow has no pending approval", ExitCode: report.ExitConflict})
	}
	if cfg.Policy.ExternalActions != "approval" {
		return failWorkflow(&result, &workflowIssue{Code: "ACTION_POLICY_DENIED", Message: "workflow policy no longer allows approval-gated external actions", ExitCode: report.ExitConflict})
	}
	if len(cfg.Policy.AllowedActions) > 0 && !containsAction(cfg.Policy.AllowedActions, snapshot.PendingApproval.Action) {
		return failWorkflow(&result, &workflowIssue{Code: "ACTION_NOT_ALLOWED", Message: fmt.Sprintf("approval action %q is not listed in workflow policy", snapshot.PendingApproval.Action), ExitCode: report.ExitConflict})
	}
	if o.ApprovalID == "" || o.ApprovalID != snapshot.PendingApproval.ID {
		return failWorkflow(&result, &workflowIssue{Code: "APPROVAL_CONFLICT", Message: "approval ID is unknown or no longer pending", ExitCode: report.ExitConflict})
	}
	decision := strings.ToLower(strings.TrimSpace(o.Decision))
	if decision == "" {
		decision = "approve"
	}
	if decision != "approve" && decision != "reject" {
		return failWorkflow(&result, issue("INVALID_ARGUMENT", "decision must be approve or reject", report.ExitConfig))
	}
	now := time.Now().UTC()
	for index := range snapshot.Approvals {
		if snapshot.Approvals[index].ID == o.ApprovalID {
			snapshot.Approvals[index].Decision = decision
			snapshot.Approvals[index].Reason = o.Reason
			snapshot.Approvals[index].DecidedAt = &now
		}
	}
	snapshot.PendingApproval = nil
	if decision == "approve" {
		snapshot.Status = workflow.StatusReady
	} else {
		snapshot.Status = workflow.StatusNeedsAttention
	}
	snapshot.UpdatedAt = now
	data := map[string]any{"operationID": opID, "approvalID": o.ApprovalID, "decision": decision, "status": snapshot.Status}
	snapshot.RecordOperation(opID, "workflow approve", data, now)
	event := workflow.NewEvent(task.Task.ID, opID, "approval", snapshot, now, map[string]any{"approvalID": o.ApprovalID, "decision": decision, "reason": o.Reason})
	if err := store.Commit(snapshot, event, workflow.CommitOptions{ClearLease: true}); err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_COMMIT_FAILED", Message: err.Error(), ExitCode: report.ExitExecution})
	}
	result.Data = data
	return result, report.ExitOK
}

func (s Service) WorkflowCancel(ctx context.Context, o WorkflowOptions) (report.Result, report.ExitCode) {
	result := report.New("workflow cancel", o.TaskID)
	task, cfg, digest, problem := s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	taskLock, err := lock.Acquire(task.Task.Root)
	if err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "TASK_LOCKED", Message: err.Error(), ExitCode: report.ExitConflict})
	}
	defer taskLock.Release()
	task, cfg, digest, problem = s.loadWorkflow(o.TasksRoot, o.TaskID)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	store := workflow.NewStore(task.Task.Root)
	snapshot, exists, problem := s.readWorkflowState(store, task.Task.ID, cfg, digest)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if !exists {
		return failWorkflow(&result, issue("WORKFLOW_NOT_STARTED", "workflow has not started", report.ExitConflict))
	}
	opID := workflowOperationID(o.OperationID)
	if previous, ok := snapshot.Operation(opID, "workflow cancel"); ok {
		result.Data = previous.Result
		return result, report.ExitOK
	}
	if problem := rejectOperationReuse(snapshot, opID, "workflow cancel"); problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := requireWorkflowDigest(snapshot, digest); problem != nil {
		return failWorkflow(&result, problem)
	}
	lease, leaseExists, problem := s.readLease(store)
	if problem != nil {
		return failWorkflow(&result, problem)
	}
	if problem := leaseTaskMismatch(lease, leaseExists, task.Task.ID); problem != nil {
		return failWorkflow(&result, problem)
	}
	if snapshot.IsTerminal() {
		return failWorkflow(&result, &workflowIssue{Code: "WORKFLOW_TERMINAL", Message: fmt.Sprintf("workflow is already %s", snapshot.Status), ExitCode: report.ExitConflict})
	}
	if snapshot.ActiveAttempt != nil {
		if !leaseExists {
			return failWorkflow(&result, &workflowIssue{Code: "LEASE_CONFLICT", Message: "active workflow lease is missing; recover before cancelling the attempt", ExitCode: report.ExitConflict})
		}
		if !time.Now().UTC().Before(lease.ExpiresAt) {
			return failWorkflow(&result, &workflowIssue{Code: "STALE_LEASE", Message: "active lease expired; recover before cancelling the active attempt", ExitCode: report.ExitConflict})
		}
		if problem := leaseMatches(lease, o.OwnerToken, task.Task.ID, time.Now().UTC()); problem != nil {
			return failWorkflow(&result, problem)
		}
		now := time.Now().UTC()
		finishAttempt(snapshot.ActiveAttempt, "cancelled", now)
		snapshot.LastAttemptID = snapshot.ActiveAttempt.ID
		snapshot.ActiveAttempt = nil
	}
	snapshot.PendingApproval = nil
	now := time.Now().UTC()
	snapshot.Status = workflow.StatusCancelled
	snapshot.UpdatedAt = now
	data := map[string]any{"operationID": opID, "status": snapshot.Status, "reason": o.Reason}
	snapshot.RecordOperation(opID, "workflow cancel", data, now)
	event := workflow.NewEvent(task.Task.ID, opID, "cancel", snapshot, now, map[string]any{"reason": o.Reason})
	if err := store.Commit(snapshot, event, workflow.CommitOptions{ClearLease: true}); err != nil {
		return failWorkflow(&result, &workflowIssue{Code: "RUNTIME_COMMIT_FAILED", Message: err.Error(), ExitCode: report.ExitExecution})
	}
	result.Data = data
	return result, report.ExitOK
}

func readReportFile(taskRoot, path string) ([]byte, error) {
	canonical, err := fsx.CanonicalExisting(path)
	if err != nil {
		return nil, err
	}
	if !fsx.Within(taskRoot, canonical) {
		return nil, fmt.Errorf("report path %q escapes task root", path)
	}
	file, err := os.Open(canonical)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, workflow.MaxReportBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > workflow.MaxReportBytes {
		return nil, fmt.Errorf("report file exceeds %d bytes", workflow.MaxReportBytes)
	}
	return raw, nil
}

func workflowEngine(value string) (string, error) {
	engine := strings.ToLower(strings.TrimSpace(value))
	if engine == "" {
		return "unknown", nil
	}
	switch engine {
	case "unknown", "codex", "claude":
		return engine, nil
	default:
		return "", fmt.Errorf("unsupported workflow engine %q; choose codex, claude, or unknown", value)
	}
}

func containsAction(actions []string, wanted string) bool {
	for _, action := range actions {
		if action == wanted {
			return true
		}
	}
	return false
}
