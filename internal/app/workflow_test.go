package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chenquan/taskflow/internal/report"
	"github.com/chenquan/taskflow/internal/workflow"
)

func writeWorkflowConfig(t *testing.T, taskRoot, taskID, command string, maxAttempts int) string {
	t.Helper()
	path := filepath.Join(taskRoot, "workflow.yaml")
	raw := `version: 1
task:
  id: TASK_ID
limits:
  max_iterations: 10
  max_duration: 1h
stages:
  - id: implement
    objective: implement and verify the change
    max_attempts: MAX_ATTEMPTS
    checks: [result]
checks:
  - id: result
    argv: ["COMMAND"]
    cwd: "repo:repo"
    timeout: 2s
policy:
  external_actions: deny
`
	raw = strings.ReplaceAll(raw, "TASK_ID", taskID)
	raw = strings.ReplaceAll(raw, "MAX_ATTEMPTS", string(rune('0'+maxAttempts)))
	raw = strings.ReplaceAll(raw, "COMMAND", command)
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func setupWorkflowTask(t *testing.T, taskID, command string, maxAttempts int) (Service, string, string) {
	t.Helper()
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: taskID, Repositories: []string{"repo=" + repo}, Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("create workflow task: code=%d result=%#v", code, result)
	}
	task, err := service.Load(tasks, taskID)
	if err != nil {
		t.Fatal(err)
	}
	writeWorkflowConfig(t, task.Task.Root, taskID, command, maxAttempts)
	return service, tasks, task.Task.Root
}

func checkpointReport(t *testing.T, taskRoot string, report workflow.AgentReport) string {
	t.Helper()
	path := filepath.Join(taskRoot, "checkpoint.json")
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func beginData(t *testing.T, result report.Result) (string, string) {
	t.Helper()
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected begin data: %#v", result.Data)
	}
	attemptID, _ := data["attemptID"].(string)
	ownerToken, _ := data["ownerToken"].(string)
	if attemptID == "" || ownerToken == "" {
		t.Fatalf("missing begin identity: %#v", data)
	}
	return attemptID, ownerToken
}

func TestWorkflowLifecycleRequiresMachineVerification(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "FLOW", "true", 2)
	if result, code := service.WorkflowValidate(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "FLOW"}); code != report.ExitOK || !result.OK {
		t.Fatalf("validate: code=%d result=%#v", code, result)
	}
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "FLOW", Engine: "codex", SessionID: "session-1", OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	attemptID, ownerToken := beginData(t, begin)
	reportPath := checkpointReport(t, taskRoot, workflow.AgentReport{
		Version:      workflow.RuntimeVersion,
		TaskID:       "FLOW",
		StageID:      "implement",
		AttemptID:    attemptID,
		SessionID:    "session-1",
		Status:       workflow.ReportReady,
		Summary:      "implementation is ready for checks",
		ChangedPaths: []string{"main.go"},
		Commands:     []workflow.CommandRecord{},
		Risks:        []string{},
		NextAction:   "verify",
	})
	checkpoint, code := service.WorkflowCheckpoint(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "FLOW", AttemptID: attemptID, OwnerToken: ownerToken, ReportPath: reportPath, OperationID: "checkpoint-1"})
	if code != report.ExitOK || !checkpoint.OK {
		t.Fatalf("checkpoint: code=%d result=%#v", code, checkpoint)
	}
	verify, code := service.WorkflowVerify(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "FLOW", AttemptID: attemptID, OwnerToken: ownerToken, OperationID: "verify-1"})
	if code != report.ExitOK || !verify.OK {
		t.Fatalf("verify: code=%d result=%#v", code, verify)
	}
	status, code := service.WorkflowStatus(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "FLOW"})
	if code != report.ExitOK || !status.OK {
		t.Fatalf("status: code=%d result=%#v", code, status)
	}
	data := status.Data.(map[string]any)
	if data["status"] != workflow.StatusCompleted {
		t.Fatalf("expected completed status, got %#v", data["status"])
	}
	for _, path := range []string{
		filepath.Join(taskRoot, ".taskflow", "workflow-state.json"),
		filepath.Join(taskRoot, ".taskflow", "workflow-events.jsonl"),
		filepath.Join(taskRoot, ".taskflow", "workflow", "attempts", attemptID, "prompt.md"),
		filepath.Join(taskRoot, ".taskflow", "workflow", "attempts", attemptID, "checks", "result.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("workflow evidence missing %s: %v", path, err)
		}
	}
}

func TestWorkflowDoesNotTreatReportAsCompletion(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "REPORT", "false", 1)
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "REPORT", OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	attemptID, ownerToken := beginData(t, begin)
	reportPath := checkpointReport(t, taskRoot, workflow.AgentReport{Version: workflow.RuntimeVersion, TaskID: "REPORT", StageID: "implement", AttemptID: attemptID, Status: workflow.ReportReady, Summary: "done", Commands: []workflow.CommandRecord{}, Risks: []string{}, NextAction: "verify"})
	if result, code := service.WorkflowCheckpoint(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "REPORT", AttemptID: attemptID, OwnerToken: ownerToken, ReportPath: reportPath, OperationID: "checkpoint-1"}); code != report.ExitOK || !result.OK {
		t.Fatalf("checkpoint: code=%d result=%#v", code, result)
	}
	status, code := service.WorkflowStatus(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "REPORT"})
	if code != report.ExitOK || status.Data.(map[string]any)["status"] != workflow.StatusVerifying {
		t.Fatalf("report incorrectly completed workflow: code=%d status=%#v", code, status)
	}
	verify, code := service.WorkflowVerify(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "REPORT", AttemptID: attemptID, OwnerToken: ownerToken, OperationID: "verify-1"})
	if code != report.ExitOK || !verify.OK {
		t.Fatalf("verify failure should be recorded: code=%d result=%#v", code, verify)
	}
	status, _ = service.WorkflowStatus(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "REPORT"})
	if status.Data.(map[string]any)["status"] != workflow.StatusNeedsAttention {
		t.Fatalf("expected needs_attention after exhausted failure, got %#v", status.Data.(map[string]any)["status"])
	}
}

func TestWorkflowOperationAndPauseResumeAreIdempotent(t *testing.T) {
	service, tasks, _ := setupWorkflowTask(t, "IDEMPOTENT", "true", 2)
	first, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "IDEMPOTENT", OperationID: "same-begin"})
	if code != report.ExitOK || !first.OK {
		t.Fatalf("first begin: code=%d result=%#v", code, first)
	}
	second, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "IDEMPOTENT", OperationID: "same-begin"})
	if code != report.ExitOK || !second.OK || second.Data.(map[string]any)["attemptID"] != first.Data.(map[string]any)["attemptID"] {
		t.Fatalf("duplicate begin not idempotent: code=%d first=%#v second=%#v", code, first, second)
	}
	operationConflict, code := service.WorkflowPause(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "IDEMPOTENT", OperationID: "same-begin"})
	if code != report.ExitConflict || operationConflict.OK || !hasDiagnostic(operationConflict.Errors, "OPERATION_CONFLICT") {
		t.Fatalf("operation ID was reused across commands: code=%d result=%#v", code, operationConflict)
	}
	attemptID, ownerToken := beginData(t, first)
	paused, code := service.WorkflowPause(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "IDEMPOTENT", OwnerToken: ownerToken, OperationID: "pause-1", Reason: "manual inspection"})
	if code != report.ExitOK || !paused.OK {
		t.Fatalf("pause: code=%d result=%#v", code, paused)
	}
	pausedAgain, code := service.WorkflowPause(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "IDEMPOTENT", OwnerToken: ownerToken, OperationID: "pause-1"})
	if code != report.ExitOK || !pausedAgain.OK {
		t.Fatalf("duplicate pause: code=%d result=%#v", code, pausedAgain)
	}
	resumed, code := service.WorkflowResume(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "IDEMPOTENT", OperationID: "resume-1"})
	if code != report.ExitOK || !resumed.OK || resumed.Data.(map[string]any)["status"] != workflow.StatusReady {
		t.Fatalf("resume: code=%d result=%#v", code, resumed)
	}
	nextBegin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "IDEMPOTENT", OperationID: "begin-2"})
	if code != report.ExitOK || !nextBegin.OK || nextBegin.Data.(map[string]any)["attemptID"] == attemptID {
		t.Fatalf("resume did not create a new attempt: code=%d result=%#v", code, nextBegin)
	}
}

func TestWorkflowControlsDoNotCreateRuntimeBeforeBegin(t *testing.T) {
	for _, name := range []string{"pause", "cancel"} {
		t.Run(name, func(t *testing.T) {
			service, tasks, taskRoot := setupWorkflowTask(t, "NOT-STARTED-"+name, "true", 1)
			var result report.Result
			var code report.ExitCode
			switch name {
			case "pause":
				result, code = service.WorkflowPause(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "NOT-STARTED-" + name, OperationID: "control-1"})
			case "cancel":
				result, code = service.WorkflowCancel(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "NOT-STARTED-" + name, OperationID: "control-1"})
			}
			if code != report.ExitConflict || result.OK || !hasDiagnostic(result.Errors, "WORKFLOW_NOT_STARTED") {
				t.Fatalf("%s before begin was accepted: code=%d result=%#v", name, code, result)
			}
			if _, err := os.Stat(filepath.Join(taskRoot, ".taskflow", "workflow-state.json")); !os.IsNotExist(err) {
				t.Fatalf("%s before begin created runtime state: %v", name, err)
			}
		})
	}
}

func TestWorkflowVerifyReplayIsIdempotentAfterTerminalTransition(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "VERIFY-REPLAY", "true", 1)
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "VERIFY-REPLAY", OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	attemptID, ownerToken := beginData(t, begin)
	reportPath := checkpointReport(t, taskRoot, workflow.AgentReport{
		Version: workflow.RuntimeVersion, TaskID: "VERIFY-REPLAY", StageID: "implement", AttemptID: attemptID,
		Status: workflow.ReportReady, Summary: "ready", Commands: []workflow.CommandRecord{}, Risks: []string{}, NextAction: "verify",
	})
	if checkpoint, code := service.WorkflowCheckpoint(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "VERIFY-REPLAY", AttemptID: attemptID, OwnerToken: ownerToken, ReportPath: reportPath, OperationID: "checkpoint-1"}); code != report.ExitOK || !checkpoint.OK {
		t.Fatalf("checkpoint: code=%d result=%#v", code, checkpoint)
	}
	first, code := service.WorkflowVerify(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "VERIFY-REPLAY", AttemptID: attemptID, OwnerToken: ownerToken, OperationID: "verify-1"})
	if code != report.ExitOK || !first.OK {
		t.Fatalf("first verify: code=%d result=%#v", code, first)
	}
	second, code := service.WorkflowVerify(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "VERIFY-REPLAY", AttemptID: "different-attempt", OwnerToken: "different-owner", OperationID: "verify-1"})
	data, _ := second.Data.(map[string]any)
	if code != report.ExitOK || !second.OK || data["status"] != string(workflow.StatusCompleted) || data["passed"] != true {
		t.Fatalf("terminal verify replay was not idempotent: code=%d result=%#v", code, second)
	}
}

func TestWorkflowActiveAttemptRequiresLeaseForPauseAndCancel(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "MISSING-LEASE", "true", 2)
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "MISSING-LEASE", OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	store := workflow.NewStore(taskRoot)
	if err := os.Remove(store.Paths.Lease); err != nil {
		t.Fatal(err)
	}
	if result, code := service.WorkflowPause(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "MISSING-LEASE", OperationID: "pause-1"}); code != report.ExitConflict || result.OK || !hasDiagnostic(result.Errors, "LEASE_CONFLICT") {
		t.Fatalf("pause without lease was accepted: code=%d result=%#v", code, result)
	}
	if result, code := service.WorkflowCancel(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "MISSING-LEASE", OperationID: "cancel-1"}); code != report.ExitConflict || result.OK || !hasDiagnostic(result.Errors, "LEASE_CONFLICT") {
		t.Fatalf("cancel without lease was accepted: code=%d result=%#v", code, result)
	}
	status, code := service.WorkflowStatus(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "MISSING-LEASE"})
	if code != report.ExitOK || status.Data.(map[string]any)["status"] != workflow.StatusUnknown || !hasDiagnostic(status.Warnings, "LEASE_MISSING") {
		t.Fatalf("missing lease was not surfaced safely: code=%d result=%#v", code, status)
	}
}

func TestWorkflowCannotPauseAndResumeAroundApproval(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "APPROVAL-PAUSE", "true", 2)
	workflowPath := filepath.Join(taskRoot, "workflow.yaml")
	raw, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "external_actions: deny", "external_actions: approval\n  allowed_actions: [manual-review]", 1))
	if err := os.WriteFile(workflowPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "APPROVAL-PAUSE", OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	attemptID, ownerToken := beginData(t, begin)
	reportPath := checkpointReport(t, taskRoot, workflow.AgentReport{
		Version: workflow.RuntimeVersion, TaskID: "APPROVAL-PAUSE", StageID: "implement", AttemptID: attemptID,
		Status: workflow.ReportNeedsApproval, Summary: "external action requested",
		Approval: &workflow.ApprovalRequest{ID: "approval-1", Action: "manual-review", Description: "review the action"},
		Commands: []workflow.CommandRecord{}, Risks: []string{}, NextAction: "wait",
	})
	checkpoint, code := service.WorkflowCheckpoint(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "APPROVAL-PAUSE", AttemptID: attemptID, OwnerToken: ownerToken, ReportPath: reportPath, OperationID: "checkpoint-1"})
	if code != report.ExitOK || checkpoint.Data.(map[string]any)["status"] != workflow.StatusAwaitingApproval {
		t.Fatalf("approval checkpoint: code=%d result=%#v", code, checkpoint)
	}
	if paused, code := service.WorkflowPause(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "APPROVAL-PAUSE", OperationID: "pause-1"}); code != report.ExitConflict || paused.OK || !hasDiagnostic(paused.Errors, "APPROVAL_REQUIRED") {
		t.Fatalf("approval was bypassed by pause: code=%d result=%#v", code, paused)
	}
	if resumed, code := service.WorkflowResume(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "APPROVAL-PAUSE", OperationID: "resume-1"}); code != report.ExitConflict || resumed.OK || !hasDiagnostic(resumed.Errors, "APPROVAL_REQUIRED") {
		t.Fatalf("approval was bypassed by resume: code=%d result=%#v", code, resumed)
	}
	status, code := service.WorkflowStatus(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "APPROVAL-PAUSE"})
	if code != report.ExitOK || status.Data.(map[string]any)["status"] != workflow.StatusAwaitingApproval {
		t.Fatalf("approval state changed unexpectedly: code=%d result=%#v", code, status)
	}
}

func TestWorkflowVerifyRejectsTamperedCheckpointReport(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "TAMPERED-REPORT", "true", 1)
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "TAMPERED-REPORT", OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	attemptID, ownerToken := beginData(t, begin)
	reportPath := checkpointReport(t, taskRoot, workflow.AgentReport{Version: workflow.RuntimeVersion, TaskID: "TAMPERED-REPORT", StageID: "implement", AttemptID: attemptID, Status: workflow.ReportReady, Summary: "ready", Commands: []workflow.CommandRecord{}, Risks: []string{}, NextAction: "verify"})
	if checkpoint, code := service.WorkflowCheckpoint(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "TAMPERED-REPORT", AttemptID: attemptID, OwnerToken: ownerToken, ReportPath: reportPath, OperationID: "checkpoint-1"}); code != report.ExitOK || !checkpoint.OK {
		t.Fatalf("checkpoint: code=%d result=%#v", code, checkpoint)
	}
	storedPath := filepath.Join(taskRoot, ".taskflow", "workflow", "attempts", attemptID, "report.json")
	stored, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatal(err)
	}
	stored = []byte(strings.Replace(string(stored), `"summary": "ready"`, `"summary": "tampered after checkpoint"`, 1))
	if err := os.WriteFile(storedPath, stored, 0644); err != nil {
		t.Fatal(err)
	}
	verified, code := service.WorkflowVerify(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "TAMPERED-REPORT", AttemptID: attemptID, OwnerToken: ownerToken, OperationID: "verify-1"})
	if code != report.ExitConfig || verified.OK || !hasDiagnostic(verified.Errors, "RUNTIME_CORRUPT") {
		t.Fatalf("tampered report was accepted: code=%d result=%#v", code, verified)
	}
	status, code := service.WorkflowStatus(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "TAMPERED-REPORT"})
	if code != report.ExitOK || status.Data.(map[string]any)["status"] != workflow.StatusVerifying || len(status.Data.(map[string]any)["events"].([]workflow.Event)) != 2 {
		t.Fatalf("tampered verification changed state: code=%d result=%#v", code, status)
	}
}

func TestWorkflowRejectsForeignLeaseWithoutClearingIt(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "FOREIGN-LEASE", "true", 1)
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "FOREIGN-LEASE", OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	store := workflow.NewStore(taskRoot)
	lease, exists, err := store.ReadLease()
	if err != nil || !exists {
		t.Fatalf("read lease: exists=%v err=%v", exists, err)
	}
	lease.TaskID = "OTHER-TASK"
	if err := workflow.SaveLease(store.Paths.Lease, lease); err != nil {
		t.Fatal(err)
	}
	result, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "FOREIGN-LEASE", OperationID: "begin-2"})
	if code != report.ExitConflict || result.OK || !hasDiagnostic(result.Errors, "LEASE_CONFLICT") {
		t.Fatalf("foreign lease was not rejected: code=%d result=%#v", code, result)
	}
	if _, exists, err := store.ReadLease(); err != nil || !exists {
		t.Fatalf("foreign lease was cleared: exists=%v err=%v", exists, err)
	}
}

func TestWorkflowApprovalAndConfigChangeFailClosed(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "APPROVAL", "true", 2)
	workflowPath := filepath.Join(taskRoot, "workflow.yaml")
	raw, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	approvalConfig := strings.Replace(string(raw), "external_actions: deny", "external_actions: approval\n  allowed_actions: [manual-review]", 1)
	if err := os.WriteFile(workflowPath, []byte(approvalConfig), 0644); err != nil {
		t.Fatal(err)
	}
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "APPROVAL", OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	attemptID, ownerToken := beginData(t, begin)
	reportPath := checkpointReport(t, taskRoot, workflow.AgentReport{Version: workflow.RuntimeVersion, TaskID: "APPROVAL", StageID: "implement", AttemptID: attemptID, Status: workflow.ReportNeedsApproval, Summary: "external action requested", Approval: &workflow.ApprovalRequest{ID: "approval-1", Action: "manual-review", Description: "review the proposed external action"}, Commands: []workflow.CommandRecord{}, Risks: []string{}, NextAction: "wait"})
	checkpoint, code := service.WorkflowCheckpoint(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "APPROVAL", AttemptID: attemptID, OwnerToken: ownerToken, ReportPath: reportPath, OperationID: "checkpoint-1"})
	if code != report.ExitOK || checkpoint.Data.(map[string]any)["status"] != workflow.StatusAwaitingApproval {
		t.Fatalf("approval checkpoint: code=%d result=%#v", code, checkpoint)
	}
	approved, code := service.WorkflowApprove(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "APPROVAL", ApprovalID: "approval-1", Decision: "approve", OperationID: "approve-1"})
	if code != report.ExitOK || approved.Data.(map[string]any)["status"] != workflow.StatusReady {
		t.Fatalf("approval: code=%d result=%#v", code, approved)
	}
	begin, code = service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "APPROVAL", OperationID: "begin-2"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin after approval: code=%d result=%#v", code, begin)
	}
	attemptID, ownerToken = beginData(t, begin)
	raw, err = os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	changedRaw := strings.Replace(string(raw), "implement and verify the change", "implement and verify the changed objective", 1)
	if err := os.WriteFile(workflowPath, []byte(changedRaw), 0644); err != nil {
		t.Fatal(err)
	}
	reportPath = checkpointReport(t, taskRoot, workflow.AgentReport{Version: workflow.RuntimeVersion, TaskID: "APPROVAL", StageID: "implement", AttemptID: attemptID, Status: workflow.ReportReady, Summary: "ready", Commands: []workflow.CommandRecord{}, Risks: []string{}, NextAction: "verify"})
	changed, code := service.WorkflowCheckpoint(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "APPROVAL", AttemptID: attemptID, OwnerToken: ownerToken, ReportPath: reportPath, OperationID: "checkpoint-2"})
	if code != report.ExitConflict || changed.OK || !hasDiagnostic(changed.Errors, "CONFIG_CHANGED") {
		t.Fatalf("expected config changed refusal: code=%d result=%#v", code, changed)
	}
}

func TestWorkflowExpiredLeaseRequiresExplicitRecovery(t *testing.T) {
	service, tasks, _ := setupWorkflowTask(t, "EXPIRED", "true", 2)
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "EXPIRED", LeaseTTL: time.Nanosecond, OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	time.Sleep(3 * time.Millisecond)
	status, code := service.WorkflowStatus(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "EXPIRED"})
	if code != report.ExitOK || status.Data.(map[string]any)["status"] != workflow.StatusUnknown {
		t.Fatalf("expected derived unknown state: code=%d status=%#v", code, status)
	}
	resume, code := service.WorkflowResume(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "EXPIRED", OperationID: "resume-1"})
	if code != report.ExitConflict || resume.OK || !hasDiagnostic(resume.Errors, "RECOVERY_REQUIRED") {
		t.Fatalf("expected explicit recovery requirement: code=%d result=%#v", code, resume)
	}
	recovered, code := service.WorkflowResume(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "EXPIRED", Recover: true, OperationID: "resume-2"})
	if code != report.ExitOK || !recovered.OK || recovered.Data.(map[string]any)["status"] != workflow.StatusReady {
		t.Fatalf("recovery failed: code=%d result=%#v", code, recovered)
	}
}

func TestWorkflowBeginAllowsOnlyOneConcurrentSession(t *testing.T) {
	service, tasks, _ := setupWorkflowTask(t, "CONCURRENT", "true", 2)
	results := make(chan report.Result, 2)
	codes := make(chan report.ExitCode, 2)
	var group sync.WaitGroup
	for _, operationID := range []string{"begin-a", "begin-b"} {
		group.Add(1)
		go func(operationID string) {
			defer group.Done()
			result, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "CONCURRENT", OperationID: operationID})
			results <- result
			codes <- code
		}(operationID)
	}
	group.Wait()
	close(results)
	close(codes)
	successes := 0
	conflicts := 0
	for result := range results {
		if result.OK {
			successes++
		} else if hasDiagnostic(result.Errors, "LEASE_CONFLICT") || hasDiagnostic(result.Errors, "TASK_LOCKED") {
			conflicts++
		} else {
			t.Fatalf("unexpected concurrent result: %#v", result)
		}
	}
	for range codes {
		// Drain the codes channel so the test also verifies both calls returned.
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent begin results: successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestWorkflowRejectsUnsupportedEngineBeforeMutation(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "ENGINE", "true", 1)
	result, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "ENGINE", Engine: "other", OperationID: "begin-1"})
	if code != report.ExitConfig || result.OK || !hasDiagnostic(result.Errors, "INVALID_ARGUMENT") {
		t.Fatalf("expected unsupported engine rejection: code=%d result=%#v", code, result)
	}
	if _, err := os.Stat(filepath.Join(taskRoot, ".taskflow", "workflow-state.json")); !os.IsNotExist(err) {
		t.Fatalf("unsupported engine created runtime state: %v", err)
	}
}

func TestWorkflowCanFinishTheAttemptThatReachesIterationLimit(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "ITERATION-LIMIT", "true", 1)
	workflowPath := filepath.Join(taskRoot, "workflow.yaml")
	raw, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workflowPath, []byte(strings.Replace(string(raw), "max_iterations: 10", "max_iterations: 1", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "ITERATION-LIMIT", OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	attemptID, ownerToken := beginData(t, begin)
	reportPath := checkpointReport(t, taskRoot, workflow.AgentReport{Version: workflow.RuntimeVersion, TaskID: "ITERATION-LIMIT", StageID: "implement", AttemptID: attemptID, Status: workflow.ReportReady, Summary: "ready", Commands: []workflow.CommandRecord{}, Risks: []string{}, NextAction: "verify"})
	checkpoint, code := service.WorkflowCheckpoint(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "ITERATION-LIMIT", AttemptID: attemptID, OwnerToken: ownerToken, ReportPath: reportPath, OperationID: "checkpoint-1"})
	if code != report.ExitOK || checkpoint.Data.(map[string]any)["status"] != workflow.StatusVerifying {
		t.Fatalf("checkpoint at iteration limit: code=%d result=%#v", code, checkpoint)
	}
	verified, code := service.WorkflowVerify(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "ITERATION-LIMIT", AttemptID: attemptID, OwnerToken: ownerToken, OperationID: "verify-1"})
	if code != report.ExitOK || verified.Data.(map[string]any)["status"] != workflow.StatusCompleted {
		t.Fatalf("verification at iteration limit: code=%d result=%#v", code, verified)
	}
}

func TestWorkflowApprovalMustBeAllowedByPolicy(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "POLICY", "true", 1)
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "POLICY", OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	attemptID, ownerToken := beginData(t, begin)
	reportPath := checkpointReport(t, taskRoot, workflow.AgentReport{Version: workflow.RuntimeVersion, TaskID: "POLICY", StageID: "implement", AttemptID: attemptID, Status: workflow.ReportNeedsApproval, Summary: "external action requested", Approval: &workflow.ApprovalRequest{ID: "approval-1", Action: "manual-review", Description: "review the proposed external action"}, Commands: []workflow.CommandRecord{}, Risks: []string{}, NextAction: "wait"})
	checkpoint, code := service.WorkflowCheckpoint(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "POLICY", AttemptID: attemptID, OwnerToken: ownerToken, ReportPath: reportPath, OperationID: "checkpoint-1"})
	if code != report.ExitConflict || checkpoint.OK || !hasDiagnostic(checkpoint.Errors, "ACTION_POLICY_DENIED") {
		t.Fatalf("expected policy denial: code=%d result=%#v", code, checkpoint)
	}
	status, code := service.WorkflowStatus(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "POLICY"})
	if code != report.ExitOK || status.Data.(map[string]any)["status"] != workflow.StatusRunning {
		t.Fatalf("policy denial changed active state: code=%d result=%#v", code, status)
	}
}

func TestDeleteCleansWorkflowArtifactsWithoutChangingWorktreeSemantics(t *testing.T) {
	service, tasks, taskRoot := setupWorkflowTask(t, "DELETE-WORKFLOW", "true", 1)
	begin, code := service.WorkflowBegin(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "DELETE-WORKFLOW", OperationID: "begin-1"})
	if code != report.ExitOK || !begin.OK {
		t.Fatalf("begin: code=%d result=%#v", code, begin)
	}
	_, ownerToken := beginData(t, begin)
	if result, code := service.WorkflowCancel(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "DELETE-WORKFLOW", OwnerToken: ownerToken, OperationID: "cancel-1", Reason: "cleanup"}); code != report.ExitOK || !result.OK {
		t.Fatalf("cancel: code=%d result=%#v", code, result)
	}
	for _, path := range []string{
		filepath.Join(taskRoot, "workflow.yaml"),
		filepath.Join(taskRoot, ".taskflow", "workflow-state.json"),
		filepath.Join(taskRoot, ".taskflow", "workflow-events.jsonl"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("workflow artifact missing before delete %s: %v", path, err)
		}
	}
	if result, code := service.Delete(context.Background(), DeleteOptions{TasksRoot: tasks, TaskID: "DELETE-WORKFLOW", Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("delete: code=%d result=%#v", code, result)
	}
	if _, err := os.Stat(taskRoot); !os.IsNotExist(err) {
		t.Fatalf("workflow task directory remains after delete: %v", err)
	}
}

func TestWorkflowWithoutConfigurationRemainsWorktreeOnly(t *testing.T) {
	repo := makeGitRepo(t)
	tasks := t.TempDir()
	service := New()
	if result, code := service.Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "NO-WORKFLOW", Repositories: []string{"repo=" + repo}, Execute: true}); code != report.ExitOK || !result.OK {
		t.Fatalf("create: code=%d result=%#v", code, result)
	}
	result, code := service.WorkflowStatus(context.Background(), WorkflowOptions{TasksRoot: tasks, TaskID: "NO-WORKFLOW"})
	if code != report.ExitConfig || result.OK || !hasDiagnostic(result.Errors, "WORKFLOW_NOT_CONFIGURED") {
		t.Fatalf("expected worktree-only task to have no workflow: code=%d result=%#v", code, result)
	}
	if _, err := os.Stat(filepath.Join(tasks, "NO-WORKFLOW", ".taskflow", "workflow-state.json")); !os.IsNotExist(err) {
		t.Fatalf("status created workflow runtime: %v", err)
	}
}

func TestWorkflowDecisionHelpers(t *testing.T) {
	now := time.Now().UTC()
	cfg := workflow.Config{Limits: workflow.Limits{MaxIterations: 2, MaxUsage: 3, MaxDuration: workflow.Duration(time.Minute)}}

	cases := []struct {
		name     string
		snapshot workflow.Snapshot
		want     string
	}{
		{name: "iterations", snapshot: workflow.Snapshot{Iteration: 2}, want: "maximum workflow iterations exhausted"},
		{name: "usage", snapshot: workflow.Snapshot{Usage: 3}, want: "maximum workflow usage exhausted"},
		{name: "duration", snapshot: workflow.Snapshot{CreatedAt: now.Add(-2 * time.Minute)}, want: "maximum workflow duration exhausted"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if reason, exceeded := budgetExceeded(test.snapshot, cfg, now); !exceeded || reason != test.want {
				t.Fatalf("budgetExceeded() = %q, %v; want %q, true", reason, exceeded, test.want)
			}
		})
	}
	if reason, exceeded := budgetExceeded(workflow.Snapshot{}, cfg, now); exceeded || reason != "" {
		t.Fatalf("zero snapshot unexpectedly exceeded budget: %q, %v", reason, exceeded)
	}
	if reason, exceeded := workflowDurationExceeded(workflow.Snapshot{CreatedAt: now.Add(-2 * time.Minute)}, cfg, now); !exceeded || reason == "" {
		t.Fatalf("duration budget was not detected: %q, %v", reason, exceeded)
	}
	if reason, exceeded := workflowDurationExceeded(workflow.Snapshot{}, cfg, now); exceeded || reason != "" {
		t.Fatalf("zero snapshot unexpectedly exceeded duration: %q, %v", reason, exceeded)
	}
	if reason, exceeded := checkpointBudgetExceeded(workflow.Snapshot{Usage: 3}, cfg, workflow.ReportProgress, now); !exceeded || reason == "" {
		t.Fatalf("progress checkpoint did not use the full budget: %q, %v", reason, exceeded)
	}
	if reason, exceeded := checkpointBudgetExceeded(workflow.Snapshot{Usage: 3}, cfg, workflow.ReportReady, now); exceeded || reason != "" {
		t.Fatalf("ready checkpoint incorrectly used iteration or usage budget: %q, %v", reason, exceeded)
	}

	if got := workflowOperationID("  operation-1  "); got != "operation-1" {
		t.Fatalf("workflowOperationID() = %q", got)
	}
	if got := workflowOperationID(" \t"); !strings.HasPrefix(got, "operation-") {
		t.Fatalf("generated operation ID = %q", got)
	}
	if err := validateLeaseTTL(-time.Second); err == nil {
		t.Fatal("negative lease TTL was accepted")
	}
	if err := validateLeaseTTL(0); err != nil {
		t.Fatalf("zero lease TTL was rejected: %v", err)
	}

	lease := workflow.Lease{TaskID: "TASK-1", OwnerToken: "owner", ExpiresAt: now.Add(time.Minute)}
	leaseCases := []struct {
		name    string
		lease   workflow.Lease
		owner   string
		wantErr string
	}{
		{name: "foreign task", lease: workflow.Lease{TaskID: "OTHER", OwnerToken: "owner", ExpiresAt: now.Add(time.Minute)}, owner: "owner", wantErr: "LEASE_CONFLICT"},
		{name: "expired", lease: workflow.Lease{TaskID: "TASK-1", OwnerToken: "owner", ExpiresAt: now}, owner: "owner", wantErr: "STALE_LEASE"},
		{name: "missing owner", lease: lease, wantErr: "LEASE_CONFLICT"},
		{name: "wrong owner", lease: lease, owner: "other", wantErr: "LEASE_CONFLICT"},
		{name: "valid", lease: lease, owner: "owner"},
	}
	for _, test := range leaseCases {
		t.Run("lease/"+test.name, func(t *testing.T) {
			problem := leaseMatches(test.lease, test.owner, "TASK-1", now)
			if test.wantErr == "" {
				if problem != nil {
					t.Fatalf("leaseMatches() = %#v", problem)
				}
				return
			}
			if problem == nil || problem.Code != test.wantErr {
				t.Fatalf("leaseMatches() = %#v; want %s", problem, test.wantErr)
			}
		})
	}
	if problem := leaseTaskMismatch(lease, false, "OTHER"); problem != nil {
		t.Fatalf("missing lease unexpectedly mismatched: %#v", problem)
	}
	if problem := leaseTaskMismatch(lease, true, "TASK-1"); problem != nil {
		t.Fatalf("matching lease mismatched: %#v", problem)
	}
	if problem := leaseTaskMismatch(workflow.Lease{TaskID: "OTHER"}, true, "TASK-1"); problem == nil || problem.Code != "LEASE_CONFLICT" {
		t.Fatalf("foreign lease mismatch = %#v", problem)
	}
	if !containsAction([]string{"review", "deploy"}, "deploy") || containsAction([]string{"review"}, "deploy") {
		t.Fatal("containsAction returned an unexpected result")
	}
}

func TestReadWorkflowStateAndLeaseErrors(t *testing.T) {
	root := t.TempDir()
	store := workflow.NewStore(root)
	cfg := workflow.Config{
		Version: workflow.ConfigVersion,
		Task:    workflow.TaskRef{ID: "TASK-1"},
		Stages:  []workflow.Stage{{ID: "stage", Objective: "test", MaxAttempts: 1}},
	}
	service := New()
	if snapshot, exists, problem := service.readWorkflowState(store, "TASK-1", cfg, "digest"); problem != nil || exists || snapshot.TaskID != "TASK-1" {
		t.Fatalf("missing state = %#v, %v, %#v", snapshot, exists, problem)
	}

	now := time.Now().UTC()
	other := workflow.NewSnapshot("OTHER", "digest", cfg, now)
	if err := store.Commit(other, workflow.NewEvent("OTHER", "operation-1", "begin", other, now, nil), workflow.CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, exists, problem := service.readWorkflowState(store, "TASK-1", cfg, "digest"); !exists || problem == nil || problem.Code != "RUNTIME_TASK_MISMATCH" {
		t.Fatalf("task mismatch = exists:%v problem:%#v", exists, problem)
	}

	invalid := workflow.NewSnapshot("TASK-1", "digest", cfg, now)
	invalid.StageID = "unknown"
	if err := store.Commit(invalid, workflow.NewEvent("TASK-1", "operation-2", "state", invalid, now, nil), workflow.CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, exists, problem := service.readWorkflowState(store, "TASK-1", cfg, "digest"); !exists || problem == nil || problem.Code != "RUNTIME_CORRUPT" {
		t.Fatalf("invalid configured state = exists:%v problem:%#v", exists, problem)
	}

	if err := os.WriteFile(store.Paths.State, []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, exists, problem := service.readWorkflowState(store, "TASK-1", cfg, "digest"); exists || problem == nil || problem.Code != "RUNTIME_CORRUPT" {
		t.Fatalf("corrupt state = exists:%v problem:%#v", exists, problem)
	}

	if lease, exists, problem := service.readLease(store); problem != nil || exists || lease.TaskID != "" {
		t.Fatalf("missing lease = %#v, %v, %#v", lease, exists, problem)
	}
	if err := os.WriteFile(store.Paths.Lease, []byte("{"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, exists, problem := service.readLease(store); exists || problem == nil || problem.Code != "RUNTIME_CORRUPT" {
		t.Fatalf("corrupt lease = exists:%v problem:%#v", exists, problem)
	}
}

func TestReadReportFileEnforcesTaskRootAndSize(t *testing.T) {
	root := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "report.json")
	if err := os.WriteFile(inside, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if raw, err := readReportFile(canonicalRoot, inside); err != nil || string(raw) != "{}" {
		t.Fatalf("readReportFile() = %q, %v", raw, err)
	}
	if _, err := readReportFile(canonicalRoot, filepath.Join(root, "missing.json")); err == nil {
		t.Fatal("missing report was accepted")
	}
	outside := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(outside, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readReportFile(canonicalRoot, outside); err == nil {
		t.Fatal("report outside task root was accepted")
	}
	large := filepath.Join(root, "large.json")
	if err := os.WriteFile(large, make([]byte, workflow.MaxReportBytes+1), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readReportFile(canonicalRoot, large); err == nil {
		t.Fatal("oversized report was accepted")
	}
}
