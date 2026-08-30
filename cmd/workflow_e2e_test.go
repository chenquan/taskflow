package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenquan/taskflow/internal/workflow"
)

func workflowConfigForCLI(taskID string) string {
	return `version: 1
task:
  id: ` + taskID + `
limits:
  max_iterations: 5
  max_duration: 1h
stages:
  - id: implement
    objective: implement and verify
    max_attempts: 2
    checks: [pass]
checks:
  - id: pass
    argv: ["true"]
    cwd: "repo:repo"
    timeout: 2s
`
}

func decodeWorkflowData(t *testing.T, output string) map[string]any {
	t.Helper()
	var envelope struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("decode workflow output: %v: %s", err, output)
	}
	if !envelope.OK {
		t.Fatalf("workflow command failed: %s", output)
	}
	return envelope.Data
}

func TestWorkflowCLIEndToEnd(t *testing.T) {
	repo := e2eGitRepo(t)
	tasks := t.TempDir()
	if output, err := runE2E(t, tasks, "create", "CLI-FLOW", "--repo", "repo="+repo, "--execute"); err != nil {
		t.Fatalf("create: %v: %s", err, output)
	}
	taskRoot := filepath.Join(tasks, "CLI-FLOW")
	if err := os.WriteFile(filepath.Join(taskRoot, "workflow.yaml"), []byte(workflowConfigForCLI("CLI-FLOW")), 0644); err != nil {
		t.Fatal(err)
	}
	if output, err := runE2E(t, tasks, "--json", "workflow", "validate", "CLI-FLOW"); err != nil {
		t.Fatalf("workflow validate: %v: %s", err, output)
	}
	statusOutput, err := runE2E(t, tasks, "--json", "workflow", "status", "CLI-FLOW")
	if err != nil {
		t.Fatalf("workflow status: %v: %s", err, statusOutput)
	}
	statusData := decodeWorkflowData(t, statusOutput)
	if statusData["status"] != string(workflow.StatusReady) || statusData["snapshotExists"] != false {
		t.Fatalf("unexpected initial status: %#v", statusData)
	}
	beginOutput, err := runE2E(t, tasks, "--json", "workflow", "begin", "CLI-FLOW", "--engine", "codex", "--session", "session-1", "--operation-id", "begin-1")
	if err != nil {
		t.Fatalf("workflow begin: %v: %s", err, beginOutput)
	}
	beginData := decodeWorkflowData(t, beginOutput)
	attemptID, _ := beginData["attemptID"].(string)
	ownerToken, _ := beginData["ownerToken"].(string)
	if attemptID == "" || ownerToken == "" {
		t.Fatalf("begin data missing identity: %#v", beginData)
	}
	reportPath := filepath.Join(taskRoot, "cli-report.json")
	reportRaw, err := json.Marshal(workflow.AgentReport{
		Version:    workflow.RuntimeVersion,
		TaskID:     "CLI-FLOW",
		StageID:    "implement",
		AttemptID:  attemptID,
		SessionID:  "session-1",
		Status:     workflow.ReportReady,
		Summary:    "ready for machine verification",
		Commands:   []workflow.CommandRecord{},
		Risks:      []string{},
		NextAction: "verify",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, reportRaw, 0644); err != nil {
		t.Fatal(err)
	}
	checkpointOutput, err := runE2E(t, tasks, "--json", "workflow", "checkpoint", "CLI-FLOW", "--attempt-id", attemptID, "--owner-token", ownerToken, "--report-file", reportPath, "--operation-id", "checkpoint-1")
	if err != nil {
		t.Fatalf("workflow checkpoint: %v: %s", err, checkpointOutput)
	}
	if checkpointData := decodeWorkflowData(t, checkpointOutput); checkpointData["status"] != string(workflow.StatusVerifying) {
		t.Fatalf("unexpected checkpoint status: %#v", checkpointData)
	}
	verifyOutput, err := runE2E(t, tasks, "--json", "workflow", "verify", "CLI-FLOW", "--attempt-id", attemptID, "--owner-token", ownerToken, "--operation-id", "verify-1")
	if err != nil {
		t.Fatalf("workflow verify: %v: %s", err, verifyOutput)
	}
	verifyData := decodeWorkflowData(t, verifyOutput)
	if verifyData["passed"] != true || verifyData["status"] != string(workflow.StatusCompleted) {
		t.Fatalf("unexpected verification data: %#v", verifyData)
	}
	finalOutput, err := runE2E(t, tasks, "--json", "workflow", "status", "CLI-FLOW")
	if err != nil || !strings.Contains(finalOutput, `"status": "completed"`) {
		t.Fatalf("final status: %v: %s", err, finalOutput)
	}
}

func TestWorkflowCLIFailsBeforeMutationForInvalidWorkflow(t *testing.T) {
	repo := e2eGitRepo(t)
	tasks := t.TempDir()
	if output, err := runE2E(t, tasks, "create", "CLI-BAD", "--repo", "repo="+repo, "--execute"); err != nil {
		t.Fatalf("create: %v: %s", err, output)
	}
	taskRoot := filepath.Join(tasks, "CLI-BAD")
	if err := os.WriteFile(filepath.Join(taskRoot, "workflow.yaml"), []byte(strings.Replace(workflowConfigForCLI("CLI-BAD"), "max_attempts: 2", "unknown_field: true", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	output, err := runE2E(t, tasks, "--json", "workflow", "begin", "CLI-BAD", "--operation-id", "begin-1")
	if err == nil || !strings.Contains(output, "INVALID_WORKFLOW_CONFIGURATION") {
		t.Fatalf("expected invalid workflow failure: %v: %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(taskRoot, ".taskflow", "workflow-state.json")); !os.IsNotExist(err) {
		t.Fatalf("invalid workflow created runtime state: %v", err)
	}
}
