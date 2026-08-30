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

func TestValidateReportRejectsInvalidBoundaries(t *testing.T) {
	base := AgentReport{
		Version:      RuntimeVersion,
		TaskID:       "TASK-1",
		StageID:      "implement",
		AttemptID:    "attempt-1",
		Status:       ReportReady,
		Summary:      "ready",
		ChangedPaths: []string{},
		Commands:     []CommandRecord{},
		Risks:        []string{},
		NextAction:   "verify",
	}
	cases := map[string]func(*AgentReport){
		"version":          func(report *AgentReport) { report.Version++ },
		"missing identity": func(report *AgentReport) { report.TaskID = "" },
		"invalid status":   func(report *AgentReport) { report.Status = ReportStatus("unknown") },
		"empty summary":    func(report *AgentReport) { report.Summary = " " },
		"large summary":    func(report *AgentReport) { report.Summary = strings.Repeat("x", MaxReportTextSize+1) },
		"empty next action": func(report *AgentReport) {
			report.NextAction = " "
		},
		"large next action": func(report *AgentReport) {
			report.NextAction = strings.Repeat("x", MaxReportTextSize+1)
		},
		"too many paths": func(report *AgentReport) {
			report.ChangedPaths = make([]string, MaxReportItems+1)
		},
		"absolute path": func(report *AgentReport) { report.ChangedPaths = []string{"/outside"} },
		"too many commands": func(report *AgentReport) {
			report.Commands = make([]CommandRecord, MaxReportItems+1)
		},
		"empty command": func(report *AgentReport) { report.Commands = []CommandRecord{{}} },
		"command NUL": func(report *AgentReport) {
			report.Commands = []CommandRecord{{Argv: []string{"go", "\x00"}}}
		},
		"too many risks": func(report *AgentReport) {
			report.Risks = make([]string, MaxReportItems+1)
		},
		"risk NUL": func(report *AgentReport) { report.Risks = []string{"bad\x00risk"} },
		"large risk": func(report *AgentReport) {
			report.Risks = []string{strings.Repeat("x", MaxReportTextSize+1)}
		},
		"approval missing": func(report *AgentReport) { report.Status = ReportNeedsApproval },
		"approval description missing": func(report *AgentReport) {
			report.Status = ReportNeedsApproval
			report.Approval = &ApprovalRequest{ID: "approval-1", Action: "review"}
		},
		"approval description large": func(report *AgentReport) {
			report.Status = ReportNeedsApproval
			report.Approval = &ApprovalRequest{ID: "approval-1", Action: "review", Description: strings.Repeat("x", MaxReportTextSize+1)}
		},
		"unexpected approval": func(report *AgentReport) {
			report.Approval = &ApprovalRequest{ID: "approval-1", Action: "review", Description: "review"}
		},
		"negative usage": func(report *AgentReport) { report.Usage = -1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			report := base
			mutate(&report)
			if err := ValidateReport(report); err == nil {
				t.Fatal("expected invalid report to be rejected")
			}
		})
	}
}

func TestLimitedBufferMarksDiscardedOutput(t *testing.T) {
	unlimited := limitedBuffer{limit: len("output")}
	if _, err := unlimited.Write([]byte("output")); err != nil || unlimited.String() != "output" || unlimited.truncated {
		t.Fatalf("unexpected unlimited buffer: %#v err=%v", unlimited, err)
	}
	zero := limitedBuffer{limit: 0}
	if _, err := zero.Write([]byte("discarded")); err != nil || !zero.truncated || zero.Len() != 0 {
		t.Fatalf("zero buffer did not record truncation: %#v err=%v", zero, err)
	}
	bounded := limitedBuffer{limit: 2}
	if _, err := bounded.Write([]byte("ok")); err != nil {
		t.Fatal(err)
	}
	if _, err := bounded.Write([]byte("more")); err != nil || bounded.String() != "ok" || !bounded.truncated {
		t.Fatalf("full buffer did not discard output: %#v err=%v", bounded, err)
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

func TestValidateLeaseRejectsInvalidBoundaries(t *testing.T) {
	now := time.Now().UTC()
	base := Lease{
		Version:    RuntimeVersion,
		TaskID:     "TASK-1",
		Engine:     "codex",
		SessionID:  "session-1",
		OwnerToken: "owner-1",
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Minute),
	}
	cases := map[string]func(*Lease){
		"version":         func(lease *Lease) { lease.Version++ },
		"missing task":    func(lease *Lease) { lease.TaskID = "" },
		"trimmed task":    func(lease *Lease) { lease.TaskID = " TASK-1" },
		"unsafe task":     func(lease *Lease) { lease.TaskID = "TASK/1" },
		"invalid engine":  func(lease *Lease) { lease.Engine = "other" },
		"missing owner":   func(lease *Lease) { lease.OwnerToken = " " },
		"owner NUL":       func(lease *Lease) { lease.OwnerToken = "owner\x00" },
		"session NUL":     func(lease *Lease) { lease.SessionID = "session\x00" },
		"missing created": func(lease *Lease) { lease.CreatedAt = time.Time{} },
		"missing expiry":  func(lease *Lease) { lease.ExpiresAt = time.Time{} },
		"expiry before creation": func(lease *Lease) {
			lease.ExpiresAt = lease.CreatedAt.Add(-time.Second)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			lease := base
			mutate(&lease)
			if err := validateLease(lease); err == nil {
				t.Fatal("invalid lease was accepted")
			}
		})
	}
	if err := validateLease(base); err != nil {
		t.Fatalf("valid lease was rejected: %v", err)
	}
}

func TestRuntimeHelperBoundaries(t *testing.T) {
	cfg := runtimeConfig()
	if _, ok := StageAt(cfg, -1); ok {
		t.Fatal("negative stage index was accepted")
	}
	if _, ok := StageAt(cfg, len(cfg.Stages)); ok {
		t.Fatal("out-of-range stage index was accepted")
	}
	if _, ok := StageAt(cfg, 0); !ok {
		t.Fatal("valid stage index was rejected")
	}

	now := time.Now().UTC()
	snapshot := NewSnapshot("TASK-1", "digest", cfg, now)
	snapshot.RecordOperation("operation-1", "workflow begin", map[string]any{"ok": true}, now)
	if operation, ok := snapshot.Operation("operation-1", "workflow begin"); !ok || operation.ID != "operation-1" {
		t.Fatalf("recorded operation = %#v, %v", operation, ok)
	}
	if _, ok := snapshot.Operation("operation-1", "workflow pause"); ok {
		t.Fatal("operation with a different command was accepted")
	}
	if _, ok := snapshot.Operation("missing", "workflow begin"); ok {
		t.Fatal("missing operation was accepted")
	}
	for _, status := range []Status{StatusReady, StatusRunning, StatusVerifying, StatusAwaitingApproval, StatusPaused, StatusUnknown, StatusNeedsAttention, StatusCompleted, StatusCancelled} {
		if !validStatus(status) {
			t.Fatalf("known status %q was rejected", status)
		}
	}
	if validStatus(Status("invalid")) {
		t.Fatal("invalid status was accepted")
	}
}

func TestValidateApprovalRejectsInvalidBoundaries(t *testing.T) {
	now := time.Now().UTC()
	base := Approval{ID: "approval-1", Action: "review", Description: "review change", RequestedAt: now}
	decided := now.Add(time.Second)
	cases := map[string]func(*Approval){
		"missing fields":        func(approval *Approval) { *approval = Approval{} },
		"NUL fields":            func(approval *Approval) { approval.ID = "approval\x00" },
		"invalid decision":      func(approval *Approval) { approval.Decision = "maybe" },
		"missing decision time": func(approval *Approval) { approval.Decision = "approve" },
		"missing decision":      func(approval *Approval) { approval.DecidedAt = &decided },
		"decision before request": func(approval *Approval) {
			approval.Decision = "reject"
			before := now.Add(-time.Second)
			approval.DecidedAt = &before
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			approval := base
			mutate(&approval)
			if err := validateApproval(approval); err == nil {
				t.Fatal("invalid approval was accepted")
			}
		})
	}
	for _, decision := range []string{"", "approve", "reject"} {
		approval := base
		approval.Decision = decision
		if decision != "" {
			approval.DecidedAt = &decided
		}
		if err := validateApproval(approval); err != nil {
			t.Fatalf("valid %q approval was rejected: %v", decision, err)
		}
	}
}

func TestValidateSnapshotRejectsInvalidBoundaries(t *testing.T) {
	now := time.Now().UTC()
	cfg := runtimeConfig()
	newSnapshot := func() Snapshot { return NewSnapshot("TASK-1", "digest", cfg, now) }
	newApproval := func() Approval {
		return Approval{ID: "approval-1", Action: "review", Description: "review change", RequestedAt: now}
	}
	decidedTime := now.Add(time.Second)
	newActiveSnapshot := func() Snapshot {
		snapshot := newSnapshot()
		snapshot.Status = StatusRunning
		snapshot.Iteration = 1
		snapshot.StageAttempts["implement"] = 1
		snapshot.ActiveAttempt = &Attempt{ID: "attempt-1", StageID: "implement", Iteration: 1, StageAttempt: 1, Status: "active", StartedAt: now, ReportPath: "report.json"}
		return snapshot
	}
	cases := map[string]func(*Snapshot){
		"version":              func(snapshot *Snapshot) { snapshot.Version++ },
		"missing identity":     func(snapshot *Snapshot) { snapshot.TaskID = "" },
		"unsafe identity":      func(snapshot *Snapshot) { snapshot.TaskID = "TASK/1" },
		"invalid status":       func(snapshot *Snapshot) { snapshot.Status = Status("invalid") },
		"missing stage":        func(snapshot *Snapshot) { snapshot.StageID = "" },
		"negative stage index": func(snapshot *Snapshot) { snapshot.StageIndex = -1 },
		"negative iteration":   func(snapshot *Snapshot) { snapshot.Iteration = -1 },
		"negative usage":       func(snapshot *Snapshot) { snapshot.Usage = -1 },
		"missing created time": func(snapshot *Snapshot) { snapshot.CreatedAt = time.Time{} },
		"updated before created": func(snapshot *Snapshot) {
			snapshot.UpdatedAt = snapshot.CreatedAt.Add(-time.Second)
		},
		"missing approvals":      func(snapshot *Snapshot) { snapshot.Approvals = nil },
		"missing operations":     func(snapshot *Snapshot) { snapshot.Operations = nil },
		"missing stage attempts": func(snapshot *Snapshot) { snapshot.StageAttempts = nil },
		"unsafe stage attempt ID": func(snapshot *Snapshot) {
			snapshot.StageAttempts = map[string]int{"stage/id": 1}
		},
		"negative stage attempts": func(snapshot *Snapshot) {
			snapshot.StageAttempts = map[string]int{"stage": -1}
		},
		"invalid approval": func(snapshot *Snapshot) {
			snapshot.Approvals = []Approval{{}}
		},
		"duplicate approvals": func(snapshot *Snapshot) {
			approval := newApproval()
			snapshot.Approvals = []Approval{approval, approval}
		},
		"incomplete operation": func(snapshot *Snapshot) {
			snapshot.Operations = map[string]OperationRecord{"operation-1": {}}
		},
		"NUL operation": func(snapshot *Snapshot) {
			snapshot.Operations = map[string]OperationRecord{"operation\x00": {ID: "operation\x00", Command: "workflow begin", CreatedAt: now}}
		},
		"unsafe last attempt": func(snapshot *Snapshot) { snapshot.LastAttemptID = "attempt/1" },
		"missing verification time": func(snapshot *Snapshot) {
			snapshot.LastVerification = &VerificationSummary{}
		},
		"invalid verification check": func(snapshot *Snapshot) {
			snapshot.LastVerification = &VerificationSummary{CompletedAt: now, CheckIDs: []string{""}}
		},
		"invalid failed check": func(snapshot *Snapshot) {
			snapshot.LastVerification = &VerificationSummary{CompletedAt: now, FailedCheck: []string{"bad\x00check"}}
		},
		"invalid pending approval": func(snapshot *Snapshot) {
			snapshot.PendingApproval = &Approval{}
		},
		"decided pending approval": func(snapshot *Snapshot) {
			approval := newApproval()
			approval.Decision = "approve"
			approval.DecidedAt = &decidedTime
			snapshot.Status = StatusAwaitingApproval
			snapshot.PendingApproval = &approval
		},
		"unrecorded pending approval": func(snapshot *Snapshot) {
			approval := newApproval()
			snapshot.Status = StatusAwaitingApproval
			snapshot.PendingApproval = &approval
		},
		"mismatched pending approval": func(snapshot *Snapshot) {
			recorded := newApproval()
			pending := recorded
			pending.Description = "different description"
			snapshot.Status = StatusAwaitingApproval
			snapshot.Approvals = []Approval{recorded}
			snapshot.PendingApproval = &pending
		},
		"active attempt incomplete": func(snapshot *Snapshot) {
			snapshot.ActiveAttempt = &Attempt{}
		},
		"active attempt identity": func(snapshot *Snapshot) {
			active := newActiveSnapshot()
			active.ActiveAttempt.ID = "attempt/1"
			*snapshot = active
		},
		"active attempt finished": func(snapshot *Snapshot) {
			active := newActiveSnapshot()
			finished := now
			active.ActiveAttempt.FinishedAt = &finished
			*snapshot = active
		},
		"active report path": func(snapshot *Snapshot) {
			active := newActiveSnapshot()
			active.ActiveAttempt.ReportPath = "../report.json"
			*snapshot = active
		},
		"active report digest length": func(snapshot *Snapshot) {
			active := newActiveSnapshot()
			active.ActiveAttempt.ReportDigest = "short"
			*snapshot = active
		},
		"active report digest encoding": func(snapshot *Snapshot) {
			active := newActiveSnapshot()
			active.ActiveAttempt.ReportDigest = strings.Repeat("z", 64)
			*snapshot = active
		},
		"active attempt status": func(snapshot *Snapshot) {
			active := newActiveSnapshot()
			active.ActiveAttempt.Status = "paused"
			*snapshot = active
		},
		"awaiting approval without request": func(snapshot *Snapshot) {
			snapshot.Status = StatusAwaitingApproval
		},
		"pending approval in wrong state": func(snapshot *Snapshot) {
			approval := newApproval()
			snapshot.Approvals = []Approval{approval}
			snapshot.PendingApproval = &approval
		},
		"running without active attempt": func(snapshot *Snapshot) { snapshot.Status = StatusRunning },
		"running with non-active attempt": func(snapshot *Snapshot) {
			active := newActiveSnapshot()
			active.ActiveAttempt.Status = "unknown"
			*snapshot = active
		},
		"unknown with active attempt": func(snapshot *Snapshot) {
			active := newActiveSnapshot()
			active.Status = StatusUnknown
			*snapshot = active
		},
		"ready with active attempt": func(snapshot *Snapshot) {
			active := newActiveSnapshot()
			active.Status = StatusReady
			*snapshot = active
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			snapshot := newSnapshot()
			mutate(&snapshot)
			if err := ValidateSnapshot(snapshot); err == nil {
				t.Fatal("invalid runtime snapshot was accepted")
			}
		})
	}
}

func TestValidateSnapshotForConfigRejectsUnknownAndExcessAttempts(t *testing.T) {
	now := time.Now().UTC()
	cfg := runtimeConfig()
	unknown := NewSnapshot("TASK-1", "digest", cfg, now)
	unknown.StageAttempts = map[string]int{"other": 1}
	if err := ValidateSnapshotForConfig(unknown, cfg); err == nil {
		t.Fatal("attempts for an unknown stage were accepted")
	}
	excess := NewSnapshot("TASK-1", "digest", cfg, now)
	excess.StageAttempts["implement"] = cfg.Stages[0].MaxAttempts + 1
	if err := ValidateSnapshotForConfig(excess, cfg); err == nil {
		t.Fatal("attempts over the stage limit were accepted")
	}
	active := NewSnapshot("TASK-1", "digest", cfg, now)
	active.Status = StatusRunning
	active.Iteration = 1
	active.StageAttempts["implement"] = 1
	active.ActiveAttempt = &Attempt{ID: "attempt-1", StageID: "other", Iteration: 1, StageAttempt: 1, Status: "active", StartedAt: now}
	if err := ValidateSnapshotForConfig(active, cfg); err == nil {
		t.Fatal("active attempt from another stage was accepted")
	}
}
