package workflow

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenquan/taskflow/internal/fsx"
)

const RuntimeVersion = 1

const (
	MaxReportBytes    = 1024 * 1024
	MaxReportItems    = 4096
	MaxReportTextSize = 64 * 1024
)

type Status string

const (
	StatusReady            Status = "ready"
	StatusRunning          Status = "running"
	StatusVerifying        Status = "verifying"
	StatusAwaitingApproval Status = "awaiting_approval"
	StatusPaused           Status = "paused"
	StatusUnknown          Status = "unknown"
	StatusNeedsAttention   Status = "needs_attention"
	StatusCompleted        Status = "completed"
	StatusCancelled        Status = "cancelled"
)

type ReportStatus string

const (
	ReportProgress      ReportStatus = "progress"
	ReportReady         ReportStatus = "ready"
	ReportBlocked       ReportStatus = "blocked"
	ReportNeedsApproval ReportStatus = "needs_approval"
)

type Snapshot struct {
	Version          int                        `json:"version"`
	TaskID           string                     `json:"taskID"`
	ConfigDigest     string                     `json:"configDigest"`
	Status           Status                     `json:"status"`
	StageIndex       int                        `json:"stageIndex"`
	StageID          string                     `json:"stageID"`
	Iteration        int                        `json:"iteration"`
	Usage            int                        `json:"usage"`
	StageAttempts    map[string]int             `json:"stageAttempts"`
	ActiveAttempt    *Attempt                   `json:"activeAttempt,omitempty"`
	LastAttemptID    string                     `json:"lastAttemptID,omitempty"`
	LastVerification *VerificationSummary       `json:"lastVerification,omitempty"`
	PendingApproval  *Approval                  `json:"pendingApproval,omitempty"`
	Approvals        []Approval                 `json:"approvals"`
	Operations       map[string]OperationRecord `json:"operations"`
	CreatedAt        time.Time                  `json:"createdAt"`
	UpdatedAt        time.Time                  `json:"updatedAt"`
}

type Attempt struct {
	ID           string     `json:"id"`
	StageID      string     `json:"stageID"`
	Iteration    int        `json:"iteration"`
	StageAttempt int        `json:"stageAttempt"`
	SessionID    string     `json:"sessionID,omitempty"`
	Status       string     `json:"status"`
	StartedAt    time.Time  `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	ReportPath   string     `json:"reportPath,omitempty"`
	ReportDigest string     `json:"reportDigest,omitempty"`
}

type VerificationSummary struct {
	Passed      bool      `json:"passed"`
	CheckIDs    []string  `json:"checkIDs"`
	FailedCheck []string  `json:"failedChecks,omitempty"`
	CompletedAt time.Time `json:"completedAt"`
}

type Approval struct {
	ID          string     `json:"id"`
	Action      string     `json:"action"`
	Description string     `json:"description"`
	Decision    string     `json:"decision,omitempty"`
	Reason      string     `json:"reason,omitempty"`
	RequestedAt time.Time  `json:"requestedAt"`
	DecidedAt   *time.Time `json:"decidedAt,omitempty"`
}

type OperationRecord struct {
	ID        string         `json:"id"`
	Command   string         `json:"command"`
	Result    map[string]any `json:"result"`
	CreatedAt time.Time      `json:"createdAt"`
}

type Event struct {
	Version     int            `json:"version"`
	ID          string         `json:"id"`
	OperationID string         `json:"operationID,omitempty"`
	Type        string         `json:"type"`
	TaskID      string         `json:"taskID"`
	Status      Status         `json:"status"`
	StageID     string         `json:"stageID,omitempty"`
	AttemptID   string         `json:"attemptID,omitempty"`
	Timestamp   time.Time      `json:"timestamp"`
	Data        map[string]any `json:"data,omitempty"`
}

type Lease struct {
	Version    int       `json:"version"`
	TaskID     string    `json:"taskID"`
	Engine     string    `json:"engine"`
	SessionID  string    `json:"sessionID,omitempty"`
	OwnerToken string    `json:"ownerToken"`
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type AgentReport struct {
	Version      int              `json:"version"`
	TaskID       string           `json:"taskID"`
	StageID      string           `json:"stageID"`
	AttemptID    string           `json:"attemptID"`
	SessionID    string           `json:"sessionID,omitempty"`
	Status       ReportStatus     `json:"status"`
	Summary      string           `json:"summary"`
	ChangedPaths []string         `json:"changedPaths"`
	Commands     []CommandRecord  `json:"commands"`
	Risks        []string         `json:"risks"`
	NextAction   string           `json:"nextAction"`
	Usage        int              `json:"usage,omitempty"`
	Approval     *ApprovalRequest `json:"approval,omitempty"`
}

type ApprovalRequest struct {
	ID          string `json:"id"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

type CommandRecord struct {
	Argv      []string `json:"argv"`
	ExitCode  int      `json:"exitCode,omitempty"`
	Succeeded bool     `json:"succeeded"`
}

type CheckResult struct {
	ID          string    `json:"id"`
	StageID     string    `json:"stageID"`
	AttemptID   string    `json:"attemptID"`
	Argv        []string  `json:"argv"`
	CWD         string    `json:"cwd"`
	StartedAt   time.Time `json:"startedAt"`
	FinishedAt  time.Time `json:"finishedAt"`
	DurationMS  int64     `json:"durationMS"`
	ExitCode    int       `json:"exitCode"`
	TimedOut    bool      `json:"timedOut"`
	Passed      bool      `json:"passed"`
	OutputLimit int       `json:"outputLimit"`
	Stdout      string    `json:"stdout,omitempty"`
	Stderr      string    `json:"stderr,omitempty"`
	OutputTrunc bool      `json:"outputTruncated,omitempty"`
	Error       string    `json:"error,omitempty"`
}

type Verification struct {
	Passed      bool          `json:"passed"`
	StageID     string        `json:"stageID"`
	AttemptID   string        `json:"attemptID"`
	Checks      []CheckResult `json:"checks"`
	CompletedAt time.Time     `json:"completedAt"`
}

type Paths struct {
	TaskRoot     string
	RuntimeRoot  string
	State        string
	Events       string
	Lease        string
	WorkflowRoot string
	Attempts     string
}

type AttemptPaths struct {
	Root   string
	Prompt string
	Report string
	Checks string
}

func NewPaths(taskRoot string) Paths {
	runtimeRoot := filepath.Join(taskRoot, ".taskflow")
	workflowRoot := filepath.Join(runtimeRoot, "workflow")
	return Paths{
		TaskRoot:     taskRoot,
		RuntimeRoot:  runtimeRoot,
		State:        filepath.Join(runtimeRoot, "workflow-state.json"),
		Events:       filepath.Join(runtimeRoot, "workflow-events.jsonl"),
		Lease:        filepath.Join(runtimeRoot, "workflow-lease.json"),
		WorkflowRoot: workflowRoot,
		Attempts:     filepath.Join(workflowRoot, "attempts"),
	}
}

func (p Paths) Attempt(id string) (AttemptPaths, error) {
	if id == "" || id == "." || id == ".." || filepath.Base(id) != id || strings.ContainsAny(id, `/\\`) {
		return AttemptPaths{}, fmt.Errorf("invalid attempt ID %q", id)
	}
	root := filepath.Join(p.Attempts, id)
	return AttemptPaths{
		Root:   root,
		Prompt: filepath.Join(root, "prompt.md"),
		Report: filepath.Join(root, "report.json"),
		Checks: filepath.Join(root, "checks"),
	}, nil
}

type Store struct{ Paths Paths }

func NewStore(taskRoot string) Store { return Store{Paths: NewPaths(taskRoot)} }

func (s Store) ReadSnapshot() (Snapshot, bool, error) {
	raw, exists, err := readOptional(s.Paths.State)
	if err != nil || !exists {
		return Snapshot{}, exists, err
	}
	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return Snapshot{}, true, &CorruptError{Path: s.Paths.State, Err: err}
	}
	if err := ValidateSnapshot(snapshot); err != nil {
		return Snapshot{}, true, &CorruptError{Path: s.Paths.State, Err: err}
	}
	return snapshot, true, nil
}

func (s Store) ReadLease() (Lease, bool, error) {
	raw, exists, err := readOptional(s.Paths.Lease)
	if err != nil || !exists {
		return Lease{}, exists, err
	}
	var lease Lease
	if err := json.Unmarshal(raw, &lease); err != nil {
		return Lease{}, true, &CorruptError{Path: s.Paths.Lease, Err: err}
	}
	if err := validateLease(lease); err != nil {
		return Lease{}, true, &CorruptError{Path: s.Paths.Lease, Err: err}
	}
	return lease, true, nil
}

func (s Store) ReadEvents(limit int) ([]Event, error) {
	file, err := os.Open(s.Paths.Events)
	if os.IsNotExist(err) {
		return []Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	all := make([]Event, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, &CorruptError{Path: s.Paths.Events, Err: err}
		}
		if event.Version != RuntimeVersion || event.ID == "" || event.Type == "" || event.TaskID == "" {
			return nil, &CorruptError{Path: s.Paths.Events, Err: errors.New("invalid event fields")}
		}
		all = append(all, event)
		if limit > 0 && len(all) > limit {
			all = all[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return all, nil
}

func (s Store) ReadReport(attemptID string) (AgentReport, bool, error) {
	paths, err := s.Paths.Attempt(attemptID)
	if err != nil {
		return AgentReport{}, false, err
	}
	raw, exists, err := readOptionalLimit(paths.Report, MaxReportBytes)
	if err != nil || !exists {
		return AgentReport{}, exists, err
	}
	report, err := DecodeReport(raw)
	if err != nil {
		return AgentReport{}, true, &CorruptError{Path: paths.Report, Err: err}
	}
	return report, true, nil
}

func (s Store) SaveReport(attemptID string, report AgentReport) error {
	paths, err := s.Paths.Attempt(attemptID)
	if err != nil {
		return err
	}
	if err := ValidateReport(report); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if len(raw)+1 > MaxReportBytes {
		return fmt.Errorf("report exceeds %d bytes", MaxReportBytes)
	}
	raw = append(raw, '\n')
	return fsx.AtomicWrite(paths.Report, raw, 0644)
}

// ReportDigest returns the digest of the canonical report value. The runtime
// stores it on the active attempt so a report edited after checkpoint cannot
// silently change the evidence used by verification.
func ReportDigest(report AgentReport) (string, error) {
	raw, err := json.Marshal(report)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func (s Store) SavePrompt(attemptID, prompt string) error {
	paths, err := s.Paths.Attempt(attemptID)
	if err != nil {
		return err
	}
	return fsx.AtomicWrite(paths.Prompt, []byte(prompt), 0644)
}

func (s Store) SaveCheckResult(result CheckResult) error {
	paths, err := s.Paths.Attempt(result.AttemptID)
	if err != nil {
		return err
	}
	if result.ID == "" {
		return fmt.Errorf("check result ID is required")
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return fsx.AtomicWrite(filepath.Join(paths.Checks, result.ID+".json"), raw, 0644)
}

type CommitOptions struct {
	Lease      *Lease
	ClearLease bool
}

// Commit replaces the snapshot and appends one event while the caller holds
// the task lock. If any part fails, the previous files are restored.
func (s Store) Commit(next Snapshot, event Event, options CommitOptions) error {
	if err := ValidateSnapshot(next); err != nil {
		return err
	}
	if event.Version == 0 {
		event.Version = RuntimeVersion
	}
	if event.ID == "" || event.Type == "" || event.TaskID == "" {
		return fmt.Errorf("event identity is required")
	}
	nextRaw, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	nextRaw = append(nextRaw, '\n')
	eventRaw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	eventRaw = append(eventRaw, '\n')
	oldState, stateExists, err := readOptional(s.Paths.State)
	if err != nil {
		return err
	}
	oldEvents, eventsExist, err := readOptional(s.Paths.Events)
	if err != nil {
		return err
	}
	oldLease, leaseExists, err := readOptional(s.Paths.Lease)
	if err != nil {
		return err
	}

	if err := fsx.AtomicWrite(s.Paths.Events, append(oldEvents, eventRaw...), 0644); err != nil {
		return err
	}
	if err := fsx.AtomicWrite(s.Paths.State, nextRaw, 0644); err != nil {
		return rollbackRuntimeFiles(s.Paths, oldState, stateExists, oldEvents, eventsExist, oldLease, leaseExists)
	}
	if options.Lease != nil {
		if err := SaveLease(s.Paths.Lease, *options.Lease); err != nil {
			return rollbackRuntimeFiles(s.Paths, oldState, stateExists, oldEvents, eventsExist, oldLease, leaseExists)
		}
	} else if options.ClearLease {
		if err := removeIfExists(s.Paths.Lease); err != nil {
			return rollbackRuntimeFiles(s.Paths, oldState, stateExists, oldEvents, eventsExist, oldLease, leaseExists)
		}
	}
	return nil
}

func SaveLease(path string, lease Lease) error {
	if err := validateLease(lease); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(lease, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return fsx.AtomicWrite(path, raw, 0644)
}

func validateLease(lease Lease) error {
	if lease.Version != RuntimeVersion {
		return fmt.Errorf("invalid lease version %d", lease.Version)
	}
	if lease.TaskID == "" || strings.TrimSpace(lease.TaskID) != lease.TaskID || strings.ContainsAny(lease.TaskID, `/\\`) || strings.ContainsRune(lease.TaskID, '\x00') {
		return fmt.Errorf("invalid lease taskID")
	}
	switch lease.Engine {
	case "unknown", "codex", "claude":
	default:
		return fmt.Errorf("invalid lease engine %q", lease.Engine)
	}
	if strings.TrimSpace(lease.OwnerToken) == "" || strings.ContainsRune(lease.OwnerToken, '\x00') || strings.ContainsRune(lease.SessionID, '\x00') {
		return fmt.Errorf("invalid lease ownership fields")
	}
	if lease.CreatedAt.IsZero() || lease.ExpiresAt.IsZero() || lease.ExpiresAt.Before(lease.CreatedAt) {
		return fmt.Errorf("invalid lease timestamps")
	}
	return nil
}

func ValidateSnapshot(snapshot Snapshot) error {
	if snapshot.Version != RuntimeVersion {
		return fmt.Errorf("unsupported runtime version %d", snapshot.Version)
	}
	if snapshot.TaskID == "" || snapshot.ConfigDigest == "" {
		return fmt.Errorf("runtime taskID and configDigest are required")
	}
	if strings.TrimSpace(snapshot.TaskID) != snapshot.TaskID || strings.ContainsAny(snapshot.TaskID, `/\\`) || strings.ContainsRune(snapshot.TaskID, '\x00') {
		return fmt.Errorf("runtime taskID must be one safe path component")
	}
	if !validStatus(snapshot.Status) {
		return fmt.Errorf("invalid runtime status %q", snapshot.Status)
	}
	if snapshot.StageID == "" {
		return fmt.Errorf("runtime stageID is required")
	}
	if snapshot.StageIndex < 0 || snapshot.Iteration < 0 || snapshot.Usage < 0 {
		return fmt.Errorf("runtime counters must not be negative")
	}
	if snapshot.CreatedAt.IsZero() || snapshot.UpdatedAt.IsZero() || snapshot.UpdatedAt.Before(snapshot.CreatedAt) {
		return fmt.Errorf("runtime timestamps are invalid")
	}
	if snapshot.Approvals == nil {
		return fmt.Errorf("runtime approvals must be initialized")
	}
	if snapshot.Operations == nil {
		return fmt.Errorf("runtime operations must be initialized")
	}
	if snapshot.StageAttempts == nil {
		return fmt.Errorf("runtime stageAttempts must be initialized")
	}
	for stageID, attempts := range snapshot.StageAttempts {
		if stageID == "" || strings.ContainsAny(stageID, `/\\`) || strings.ContainsRune(stageID, '\x00') {
			return fmt.Errorf("runtime stageAttempts contains an invalid stage ID")
		}
		if attempts < 0 {
			return fmt.Errorf("runtime stage attempt counts must not be negative")
		}
	}
	approvalIDs := make(map[string]struct{}, len(snapshot.Approvals))
	for _, approval := range snapshot.Approvals {
		if err := validateApproval(approval); err != nil {
			return err
		}
		if _, exists := approvalIDs[approval.ID]; exists {
			return fmt.Errorf("runtime approvals contain duplicate ID %q", approval.ID)
		}
		approvalIDs[approval.ID] = struct{}{}
	}
	for operationID, operation := range snapshot.Operations {
		if operationID == "" || operation.ID != operationID || operation.Command == "" || operation.CreatedAt.IsZero() {
			return fmt.Errorf("runtime operation %q is incomplete", operationID)
		}
		if strings.ContainsRune(operationID, '\x00') || strings.ContainsRune(operation.Command, '\x00') {
			return fmt.Errorf("runtime operation %q contains a NUL byte", operationID)
		}
	}
	if snapshot.LastAttemptID != "" && (strings.ContainsAny(snapshot.LastAttemptID, `/\\`) || strings.ContainsRune(snapshot.LastAttemptID, '\x00')) {
		return fmt.Errorf("runtime lastAttemptID is invalid")
	}
	if snapshot.LastVerification != nil {
		if snapshot.LastVerification.CompletedAt.IsZero() {
			return fmt.Errorf("runtime last verification timestamp is required")
		}
		for _, checkID := range snapshot.LastVerification.CheckIDs {
			if checkID == "" || strings.ContainsRune(checkID, '\x00') {
				return fmt.Errorf("runtime last verification contains an invalid check ID")
			}
		}
		for _, checkID := range snapshot.LastVerification.FailedCheck {
			if checkID == "" || strings.ContainsRune(checkID, '\x00') {
				return fmt.Errorf("runtime last verification contains an invalid failed check ID")
			}
		}
	}
	if snapshot.PendingApproval != nil {
		if err := validateApproval(*snapshot.PendingApproval); err != nil {
			return fmt.Errorf("runtime pending approval: %w", err)
		}
		if snapshot.PendingApproval.Decision != "" || snapshot.PendingApproval.DecidedAt != nil {
			return fmt.Errorf("runtime pending approval is already decided")
		}
		if _, exists := approvalIDs[snapshot.PendingApproval.ID]; !exists {
			return fmt.Errorf("runtime pending approval is not recorded")
		}
		for _, approval := range snapshot.Approvals {
			if approval.ID == snapshot.PendingApproval.ID && (approval.Action != snapshot.PendingApproval.Action || approval.Description != snapshot.PendingApproval.Description || approval.Decision != "" || approval.DecidedAt != nil) {
				return fmt.Errorf("runtime pending approval does not match its record")
			}
		}
	}
	if snapshot.ActiveAttempt != nil {
		attempt := snapshot.ActiveAttempt
		if attempt.ID == "" || attempt.StageID == "" || attempt.Status == "" || attempt.Iteration <= 0 || attempt.StageAttempt <= 0 || attempt.StartedAt.IsZero() {
			return fmt.Errorf("active attempt is incomplete")
		}
		if strings.ContainsAny(attempt.ID, `/\\`) || strings.ContainsRune(attempt.ID, '\x00') || strings.ContainsAny(attempt.StageID, `/\\`) || strings.ContainsRune(attempt.StageID, '\x00') || strings.ContainsRune(attempt.SessionID, '\x00') {
			return fmt.Errorf("active attempt identity is invalid")
		}
		if attempt.FinishedAt != nil {
			return fmt.Errorf("active attempt must not have a finished timestamp")
		}
		if attempt.ReportPath != "" {
			clean := filepath.Clean(filepath.FromSlash(attempt.ReportPath))
			if filepath.IsAbs(attempt.ReportPath) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.ContainsRune(attempt.ReportPath, '\x00') {
				return fmt.Errorf("active attempt report path is invalid")
			}
		}
		if attempt.ReportDigest != "" {
			if len(attempt.ReportDigest) != sha256.Size*2 {
				return fmt.Errorf("active attempt report digest is invalid")
			}
			if _, err := hex.DecodeString(attempt.ReportDigest); err != nil {
				return fmt.Errorf("active attempt report digest is invalid: %w", err)
			}
		}
		if attempt.Status != "active" && attempt.Status != "unknown" {
			return fmt.Errorf("active attempt status %q is invalid", attempt.Status)
		}
	}
	if snapshot.Status == StatusAwaitingApproval && snapshot.PendingApproval == nil {
		return fmt.Errorf("awaiting approval state requires a pending approval")
	}
	if snapshot.Status != StatusAwaitingApproval && snapshot.PendingApproval != nil {
		return fmt.Errorf("pending approval is only valid in awaiting_approval state")
	}
	switch snapshot.Status {
	case StatusRunning, StatusVerifying:
		if snapshot.ActiveAttempt == nil || snapshot.ActiveAttempt.Status != "active" {
			return fmt.Errorf("%s state requires an active attempt", snapshot.Status)
		}
	case StatusUnknown:
		if snapshot.ActiveAttempt != nil && snapshot.ActiveAttempt.Status != "unknown" {
			return fmt.Errorf("unknown state requires an unknown active attempt")
		}
	default:
		if snapshot.ActiveAttempt != nil {
			return fmt.Errorf("%s state must not have an active attempt", snapshot.Status)
		}
	}
	return nil
}

// ValidateSnapshotForConfig validates invariants that depend on the current
// declared stage list. Callers should use it only after the snapshot digest
// has been compared with the current workflow configuration.
func ValidateSnapshotForConfig(snapshot Snapshot, cfg Config) error {
	if err := ValidateSnapshot(snapshot); err != nil {
		return err
	}
	stage, ok := StageAt(cfg, snapshot.StageIndex)
	if !ok || stage.ID != snapshot.StageID {
		return fmt.Errorf("runtime stage does not match workflow configuration")
	}
	knownStages := make(map[string]Stage, len(cfg.Stages))
	for _, configured := range cfg.Stages {
		knownStages[configured.ID] = configured
	}
	for stageID, attempts := range snapshot.StageAttempts {
		configured, exists := knownStages[stageID]
		if !exists {
			return fmt.Errorf("runtime contains attempts for unknown stage %q", stageID)
		}
		if attempts > configured.MaxAttempts {
			return fmt.Errorf("runtime attempts for stage %q exceed the configured limit", stageID)
		}
	}
	if snapshot.ActiveAttempt != nil {
		active := snapshot.ActiveAttempt
		if active.StageID != snapshot.StageID || active.Iteration != snapshot.Iteration || active.StageAttempt != snapshot.StageAttempts[active.StageID] {
			return fmt.Errorf("active attempt does not match runtime stage or counters")
		}
		if active.StageAttempt > stage.MaxAttempts {
			return fmt.Errorf("active attempt exceeds the configured stage limit")
		}
	}
	return nil
}

func validateApproval(approval Approval) error {
	if approval.ID == "" || approval.Action == "" || strings.TrimSpace(approval.Description) == "" || approval.RequestedAt.IsZero() {
		return fmt.Errorf("runtime approval is incomplete")
	}
	if strings.ContainsRune(approval.ID, '\x00') || strings.ContainsRune(approval.Action, '\x00') || strings.ContainsRune(approval.Description, '\x00') {
		return fmt.Errorf("runtime approval contains a NUL byte")
	}
	if approval.Decision != "" && approval.Decision != "approve" && approval.Decision != "reject" {
		return fmt.Errorf("runtime approval has invalid decision %q", approval.Decision)
	}
	if approval.Decision == "" && approval.DecidedAt != nil {
		return fmt.Errorf("runtime approval has a decision timestamp without a decision")
	}
	if approval.Decision != "" && approval.DecidedAt == nil {
		return fmt.Errorf("runtime approval decision timestamp is required")
	}
	if approval.DecidedAt != nil && approval.DecidedAt.Before(approval.RequestedAt) {
		return fmt.Errorf("runtime approval decision predates its request")
	}
	return nil
}

func NewSnapshot(taskID, digest string, cfg Config, now time.Time) Snapshot {
	stageID := ""
	if stage, ok := StageAt(cfg, 0); ok {
		stageID = stage.ID
	}
	return Snapshot{
		Version:       RuntimeVersion,
		TaskID:        taskID,
		ConfigDigest:  digest,
		Status:        StatusReady,
		StageIndex:    0,
		StageID:       stageID,
		StageAttempts: map[string]int{},
		Approvals:     []Approval{},
		Operations:    map[string]OperationRecord{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func NewEvent(taskID, operationID, eventType string, snapshot Snapshot, now time.Time, data map[string]any) Event {
	return Event{
		Version:     RuntimeVersion,
		ID:          NewID("event"),
		OperationID: operationID,
		Type:        eventType,
		TaskID:      taskID,
		Status:      snapshot.Status,
		StageID:     snapshot.StageID,
		AttemptID:   attemptID(snapshot),
		Timestamp:   now,
		Data:        data,
	}
}

func attemptID(snapshot Snapshot) string {
	if snapshot.ActiveAttempt == nil {
		return ""
	}
	return snapshot.ActiveAttempt.ID
}

func (s Snapshot) Operation(id, command string) (OperationRecord, bool) {
	if id == "" {
		return OperationRecord{}, false
	}
	operation, ok := s.Operations[id]
	if !ok || operation.Command != command {
		return OperationRecord{}, false
	}
	return operation, true
}

func (s *Snapshot) RecordOperation(id, command string, result map[string]any, now time.Time) {
	if s.Operations == nil {
		s.Operations = map[string]OperationRecord{}
	}
	s.Operations[id] = OperationRecord{ID: id, Command: command, Result: result, CreatedAt: now}
}

func (s Snapshot) IsTerminal() bool {
	return s.Status == StatusCompleted || s.Status == StatusCancelled
}

func validStatus(status Status) bool {
	switch status {
	case StatusReady, StatusRunning, StatusVerifying, StatusAwaitingApproval, StatusPaused, StatusUnknown, StatusNeedsAttention, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}

func ValidateReport(report AgentReport) error {
	if report.Version != RuntimeVersion {
		return fmt.Errorf("unsupported report version %d", report.Version)
	}
	if report.TaskID == "" || report.StageID == "" || report.AttemptID == "" {
		return fmt.Errorf("report taskID, stageID, and attemptID are required")
	}
	switch report.Status {
	case ReportProgress, ReportReady, ReportBlocked, ReportNeedsApproval:
	default:
		return fmt.Errorf("invalid report status %q", report.Status)
	}
	if strings.TrimSpace(report.Summary) == "" {
		return fmt.Errorf("report summary is required")
	}
	if len(report.Summary) > MaxReportTextSize {
		return fmt.Errorf("report summary exceeds %d bytes", MaxReportTextSize)
	}
	if strings.TrimSpace(report.NextAction) == "" {
		return fmt.Errorf("report nextAction is required")
	}
	if len(report.NextAction) > MaxReportTextSize {
		return fmt.Errorf("report nextAction exceeds %d bytes", MaxReportTextSize)
	}
	if len(report.ChangedPaths) > MaxReportItems {
		return fmt.Errorf("report has too many changed paths")
	}
	for _, path := range report.ChangedPaths {
		if err := validateChangedPath(path); err != nil {
			return err
		}
	}
	if len(report.Commands) > MaxReportItems {
		return fmt.Errorf("report has too many command records")
	}
	for index, command := range report.Commands {
		if len(command.Argv) == 0 || strings.TrimSpace(command.Argv[0]) == "" {
			return fmt.Errorf("report command %d argv must contain an executable", index)
		}
		for _, arg := range command.Argv {
			if strings.ContainsRune(arg, '\x00') {
				return fmt.Errorf("report command %d contains a NUL byte", index)
			}
		}
	}
	if len(report.Risks) > MaxReportItems {
		return fmt.Errorf("report has too many risks")
	}
	for _, risk := range report.Risks {
		if strings.ContainsRune(risk, '\x00') {
			return fmt.Errorf("report risk contains a NUL byte")
		}
		if len(risk) > MaxReportTextSize {
			return fmt.Errorf("report risk exceeds %d bytes", MaxReportTextSize)
		}
	}
	if report.Status == ReportNeedsApproval {
		if report.Approval == nil || report.Approval.ID == "" || report.Approval.Action == "" {
			return fmt.Errorf("approval details are required for needs_approval report")
		}
		if strings.TrimSpace(report.Approval.Description) == "" {
			return fmt.Errorf("approval description is required")
		}
		if len(report.Approval.Description) > MaxReportTextSize {
			return fmt.Errorf("approval description exceeds %d bytes", MaxReportTextSize)
		}
	} else if report.Approval != nil {
		return fmt.Errorf("approval details are only allowed for needs_approval report")
	}
	if report.Usage < 0 {
		return fmt.Errorf("report usage must not be negative")
	}
	return nil
}

func validateChangedPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("report changed path must not be empty")
	}
	if strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("report changed path contains a NUL byte")
	}
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return fmt.Errorf("report changed path %q must be relative", path)
	}
	clean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(path, "\\", "/")))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("report changed path %q escapes the worktree", path)
	}
	if len(path) > MaxReportTextSize {
		return fmt.Errorf("report changed path exceeds %d bytes", MaxReportTextSize)
	}
	return nil
}

func DecodeReport(raw []byte) (AgentReport, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var report AgentReport
	if err := decoder.Decode(&report); err != nil {
		return AgentReport{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return AgentReport{}, fmt.Errorf("multiple JSON values are not supported")
		}
		return AgentReport{}, fmt.Errorf("multiple JSON values are not supported: %w", err)
	}
	return report, nil
}

func NewID(prefix string) string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return prefix + "-" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
}

type CorruptError struct {
	Path string
	Err  error
}

func (e *CorruptError) Error() string {
	return fmt.Sprintf("corrupt runtime file %s: %v", e.Path, e.Err)
}
func (e *CorruptError) Unwrap() error { return e.Err }

func readOptional(path string) ([]byte, bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

func readOptionalLimit(path string, limit int) ([]byte, bool, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, true, err
	}
	if len(raw) > limit {
		return nil, true, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return raw, true, nil
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func restoreFile(path string, raw []byte, exists bool) error {
	if !exists {
		return removeIfExists(path)
	}
	return fsx.AtomicWrite(path, raw, 0644)
}

func rollbackRuntimeFiles(paths Paths, oldState []byte, stateExists bool, oldEvents []byte, eventsExist bool, oldLease []byte, leaseExists bool) error {
	var rollbackErr error
	if err := restoreFile(paths.State, oldState, stateExists); err != nil {
		rollbackErr = err
	}
	if err := restoreFile(paths.Events, oldEvents, eventsExist); err != nil && rollbackErr == nil {
		rollbackErr = err
	}
	if err := restoreFile(paths.Lease, oldLease, leaseExists); err != nil && rollbackErr == nil {
		rollbackErr = err
	}
	if rollbackErr != nil {
		return fmt.Errorf("runtime commit failed and rollback failed: %w", rollbackErr)
	}
	return fmt.Errorf("runtime commit failed")
}
