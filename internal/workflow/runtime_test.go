package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runtimeConfig() Config {
	return Normalize(Config{
		Version: ConfigVersion,
		Task:    TaskRef{ID: "TASK-1"},
		Stages:  []Stage{{ID: "implement", Objective: "implement", MaxAttempts: 2, Checks: []string{"tests"}}},
		Checks:  []Check{{ID: "tests", Argv: []string{"true"}, CWD: "task", Timeout: Duration(time.Minute)}},
	})
}

func TestStoreCommitPersistsSnapshotEventsAndEvidence(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	now := time.Now().UTC().Truncate(time.Second)
	cfg := runtimeConfig()
	snapshot := NewSnapshot("TASK-1", "digest", cfg, now)
	attemptID := NewID("attempt")
	snapshot.Status = StatusRunning
	snapshot.StageAttempts["implement"] = 1
	snapshot.Iteration = 1
	snapshot.LastAttemptID = attemptID
	snapshot.ActiveAttempt = &Attempt{ID: attemptID, StageID: "implement", Iteration: 1, StageAttempt: 1, Status: "active", StartedAt: now, ReportPath: filepath.Join(".taskflow", "workflow", "attempts", attemptID, "report.json")}
	event := NewEvent("TASK-1", "operation-1", "begin", snapshot, now, map[string]any{"test": true})
	if err := store.Commit(snapshot, event, CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	loaded, exists, err := store.ReadSnapshot()
	if err != nil || !exists || loaded.ActiveAttempt == nil || loaded.ActiveAttempt.ID != attemptID {
		t.Fatalf("loaded snapshot=%#v exists=%v err=%v", loaded, exists, err)
	}
	events, err := store.ReadEvents(10)
	if err != nil || len(events) != 1 || events[0].Type != "begin" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	report := AgentReport{Version: RuntimeVersion, TaskID: "TASK-1", StageID: "implement", AttemptID: attemptID, Status: ReportReady, Summary: "ready", ChangedPaths: []string{"main.go"}, Commands: []CommandRecord{}, Risks: []string{}, NextAction: "verify"}
	if err := store.SavePrompt(attemptID, "prompt"); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveReport(attemptID, report); err != nil {
		t.Fatal(err)
	}
	readReport, exists, err := store.ReadReport(attemptID)
	if err != nil || !exists || readReport.Summary != "ready" {
		t.Fatalf("report=%#v exists=%v err=%v", readReport, exists, err)
	}
}

func TestStoreCommitRollbackPreservesPreviousSnapshotAndEvents(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	cfg := runtimeConfig()
	now := time.Now().UTC()
	initial := NewSnapshot("TASK-1", "digest", cfg, now)
	if err := store.Commit(initial, NewEvent("TASK-1", "initial", "init", initial, now, nil), CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.ReadFile(store.Paths.State)
	if err != nil {
		t.Fatal(err)
	}
	eventsBefore, err := os.ReadFile(store.Paths.Events)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(store.Paths.State+"-blocked", 0755); err != nil {
		t.Fatal(err)
	}
	blocked := store
	blocked.Paths.State = store.Paths.State + "-blocked"
	next := initial
	next.Status = StatusRunning
	next.UpdatedAt = now.Add(time.Second)
	err = blocked.Commit(next, NewEvent("TASK-1", "failed", "should_rollback", next, now, nil), CommitOptions{})
	if err == nil {
		t.Fatal("expected commit failure")
	}
	if got, _ := os.ReadFile(store.Paths.State); string(got) != string(stateBefore) {
		t.Fatal("previous snapshot changed")
	}
	if got, _ := os.ReadFile(store.Paths.Events); string(got) != string(eventsBefore) {
		t.Fatal("previous events changed")
	}
}

func TestDecodeReportRejectsUnknownFieldsAndMultipleValues(t *testing.T) {
	if _, err := DecodeReport([]byte(`{"version":1,"taskID":"t","stageID":"s","attemptID":"a","status":"ready","summary":"ok","unknown":true}`)); err == nil {
		t.Fatal("expected unknown field rejection")
	}
	if _, err := DecodeReport([]byte(`{"version":1,"taskID":"t","stageID":"s","attemptID":"a","status":"ready","summary":"ok"} {}`)); err == nil {
		t.Fatal("expected multiple value rejection")
	}
}

func TestStoreBoundsEventReadsAndReports(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	cfg := runtimeConfig()
	now := time.Now().UTC().Truncate(time.Second)
	for index := 0; index < 3; index++ {
		snapshot := NewSnapshot("TASK-1", "digest", cfg, now.Add(time.Duration(index)*time.Second))
		if err := store.Commit(snapshot, NewEvent("TASK-1", "operation-"+string(rune('1'+index)), "event", snapshot, snapshot.UpdatedAt, nil), CommitOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	events, err := store.ReadEvents(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].OperationID != "operation-2" || events[1].OperationID != "operation-3" {
		t.Fatalf("bounded events=%#v", events)
	}

	attemptID := "attempt-large"
	paths, err := store.Paths.Attempt(attemptID)
	if err != nil {
		t.Fatal(err)
	}
	largeReport := AgentReport{
		Version:      RuntimeVersion,
		TaskID:       "TASK-1",
		StageID:      "implement",
		AttemptID:    attemptID,
		Status:       ReportReady,
		Summary:      "ready",
		ChangedPaths: make([]string, MaxReportItems),
		Commands:     []CommandRecord{},
		Risks:        []string{},
		NextAction:   "verify",
	}
	for index := range largeReport.ChangedPaths {
		largeReport.ChangedPaths[index] = strings.Repeat("a", 256)
	}
	if err := store.SaveReport(attemptID, largeReport); err == nil {
		t.Fatal("expected oversized report to be rejected")
	}
	if _, err := os.Stat(paths.Report); !os.IsNotExist(err) {
		t.Fatalf("oversized report should not be persisted, stat error=%v", err)
	}

	if err := os.MkdirAll(filepath.Dir(paths.Report), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Report, []byte(strings.Repeat("x", MaxReportBytes+1)), 0644); err != nil {
		t.Fatal(err)
	}
	if _, exists, err := store.ReadReport(attemptID); err == nil || !exists {
		t.Fatalf("expected oversized report read failure, exists=%v err=%v", exists, err)
	}
}

func TestSaveLeaseRejectsInvalidFields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	lease := Lease{
		Version:    RuntimeVersion,
		TaskID:     "TASK-1",
		Engine:     "invalid",
		OwnerToken: "owner",
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Minute),
	}
	if err := SaveLease(filepath.Join(t.TempDir(), "lease.json"), lease); err == nil {
		t.Fatal("expected invalid engine to be rejected")
	}
	lease.Engine = "codex"
	lease.SessionID = "session-1"
	path := filepath.Join(t.TempDir(), "lease.json")
	if err := SaveLease(path, lease); err != nil {
		t.Fatal(err)
	}
	loaded, exists, err := (Store{Paths: Paths{Lease: path}}).ReadLease()
	if err != nil || !exists || loaded.TaskID != lease.TaskID || loaded.SessionID != lease.SessionID {
		t.Fatalf("loaded lease=%#v exists=%v err=%v", loaded, exists, err)
	}
}

func TestValidateReportRejectsUnsafeMetadata(t *testing.T) {
	base := AgentReport{
		Version:      RuntimeVersion,
		TaskID:       "TASK-1",
		StageID:      "implement",
		AttemptID:    "attempt-1",
		Status:       ReportReady,
		Summary:      "ready",
		ChangedPaths: []string{"main.go"},
		Commands:     []CommandRecord{{Argv: []string{"go", "test"}}},
		Risks:        []string{},
		NextAction:   "verify",
	}
	for name, mutate := range map[string]func(*AgentReport){
		"path traversal": func(report *AgentReport) { report.ChangedPaths = []string{"../outside"} },
		"empty command":  func(report *AgentReport) { report.Commands = []CommandRecord{{}} },
		"missing next action": func(report *AgentReport) {
			report.NextAction = " "
		},
	} {
		t.Run(name, func(t *testing.T) {
			report := base
			mutate(&report)
			if err := ValidateReport(report); err == nil {
				t.Fatal("expected invalid report")
			}
		})
	}
}

func TestPathsRejectAttemptTraversal(t *testing.T) {
	_, err := NewPaths(t.TempDir()).Attempt(filepath.Join("..", "escape"))
	if err == nil || err.Error() == "" {
		t.Fatalf("expected attempt traversal rejection: %v", err)
	}
}

func TestValidateSnapshotRejectsInconsistentRuntimeStates(t *testing.T) {
	now := time.Now().UTC()
	base := NewSnapshot("TASK-1", "digest", runtimeConfig(), now)
	active := &Attempt{ID: "attempt-1", StageID: "implement", Iteration: 1, StageAttempt: 1, Status: "active", StartedAt: now, ReportPath: ".taskflow/workflow/attempts/attempt-1/report.json"}
	cases := map[string]func(*Snapshot){
		"running without active attempt": func(snapshot *Snapshot) { snapshot.Status = StatusRunning },
		"ready with active attempt": func(snapshot *Snapshot) {
			snapshot.Status = StatusReady
			snapshot.Iteration = 1
			snapshot.StageAttempts["implement"] = 1
			snapshot.LastAttemptID = active.ID
			snapshot.ActiveAttempt = active
		},
		"awaiting approval without request": func(snapshot *Snapshot) { snapshot.Status = StatusAwaitingApproval },
		"operation identity mismatch": func(snapshot *Snapshot) {
			snapshot.Operations["operation-1"] = OperationRecord{ID: "other", Command: "workflow begin", CreatedAt: now}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			snapshot := base
			snapshot.StageAttempts = map[string]int{}
			snapshot.Operations = map[string]OperationRecord{}
			mutate(&snapshot)
			if err := ValidateSnapshot(snapshot); err == nil {
				t.Fatal("expected inconsistent runtime state to be rejected")
			}
		})
	}
}

func TestValidateSnapshotForConfigRejectsStageMismatch(t *testing.T) {
	now := time.Now().UTC()
	cfg := runtimeConfig()
	snapshot := NewSnapshot("TASK-1", "digest", cfg, now)
	snapshot.StageID = "other"
	if err := ValidateSnapshotForConfig(snapshot, cfg); err == nil {
		t.Fatal("expected stage mismatch to be rejected")
	}
}
