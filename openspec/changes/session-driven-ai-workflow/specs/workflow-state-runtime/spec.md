## ADDED Requirements

### Requirement: Workflow runtime state SHALL be durable and auditable
The system SHALL persist a versioned workflow snapshot, an append-only JSONL event log, per-attempt evidence, and a workflow lease under the task's `.taskflow/` directory. The snapshot and runtime records MUST identify the task and workflow configuration digest.

#### Scenario: First workflow attempt
- **WHEN** `workflow begin` successfully starts the first attempt
- **THEN** the system persists the active stage, attempt identifier, configuration digest, lease, and corresponding begin event

#### Scenario: Event history is inspected
- **WHEN** the user requests workflow status after multiple iterations
- **THEN** the result includes the current snapshot and enough recent event/evidence references to explain the latest transition

### Requirement: Runtime state writes SHALL be atomic and lock-protected
Every state transition SHALL acquire the task lock, validate the current snapshot before mutation, write the replacement snapshot atomically, and append the transition event. A failed write MUST leave the previous valid snapshot intact.

#### Scenario: Concurrent state mutation
- **WHEN** two sessions attempt to advance the same task concurrently
- **THEN** only one transition succeeds and the other receives a task or lease conflict without corrupting state

#### Scenario: State write failure
- **WHEN** a runtime snapshot replacement fails
- **THEN** the system preserves the previous snapshot and reports an execution error without appending a false success event

### Requirement: Workflow transitions SHALL be explicit and evidence-based
The runtime SHALL support `ready`, `running`, `verifying`, `awaiting_approval`, `paused`, `unknown`, `needs_attention`, `completed`, and `cancelled` states. Only valid transitions for the current state, stage, attempt, lease, and configuration digest SHALL be accepted.

#### Scenario: Verification passes
- **WHEN** all required checks for the active stage exit successfully and the stage report is valid
- **THEN** the runtime advances to the next declared stage or enters `completed` when no stage remains

#### Scenario: Model claims success without checks
- **WHEN** an Agent checkpoint claims completion but required checks have not succeeded
- **THEN** the runtime keeps the workflow non-terminal and rejects the completion transition

### Requirement: Attempts and operations SHALL be idempotent
The runtime SHALL assign stable attempt and operation identifiers. Repeating a begin, checkpoint, verify, pause, resume, approval, or cancellation operation with the same valid identifier SHALL return the existing result without duplicating effects.

#### Scenario: Duplicate checkpoint
- **WHEN** the Skill submits the same checkpoint after a network or process retry
- **THEN** the runtime returns the original checkpoint result and appends no duplicate transition

#### Scenario: Stale attempt checkpoint
- **WHEN** a checkpoint references an attempt that is no longer active
- **THEN** the runtime rejects it with an attempt conflict and leaves the current state unchanged

### Requirement: Leases SHALL protect interactive ownership and support safe recovery
The runtime SHALL create an expiring lease for an active workflow session and SHALL require a valid owner token for active-attempt mutations. An expired lease SHALL permit inspection but SHALL require explicit recovery before continuation.

#### Scenario: Valid lease renewal
- **WHEN** the active session calls checkpoint or verify before lease expiry
- **THEN** the runtime validates the owner token and refreshes the lease expiry atomically with the operation

#### Scenario: Expired lease
- **WHEN** a new session attempts to continue after the previous lease expired
- **THEN** the runtime returns a stale-lease condition and requires recovery inspection or explicit resume before beginning work

### Requirement: Interrupted attempts SHALL fail closed
If the runtime cannot establish whether an active Agent operation completed, it SHALL mark the attempt `unknown`, preserve all worktree and evidence files, and SHALL NOT automatically replay a potentially non-idempotent action.

#### Scenario: Session terminates before checkpoint
- **WHEN** the Agent session terminates after `begin` but before checkpoint
- **THEN** the next status or recovery operation reports `unknown` and retains the worktree for inspection

#### Scenario: Recovery confirms no active operation
- **WHEN** a user explicitly resumes an `unknown` attempt after inspecting the worktree and evidence
- **THEN** the runtime creates a new attempt or resumes according to the recorded recovery decision without deleting prior evidence

### Requirement: Configuration changes SHALL invalidate active execution
The runtime SHALL compare the current normalized workflow configuration digest with the digest recorded at begin. A mismatch SHALL stop active execution and return `CONFIG_CHANGED` until the user explicitly starts a new execution context.

#### Scenario: Workflow file changes during a run
- **WHEN** `workflow.yaml` changes after an attempt begins
- **THEN** checkpoint, verify, and resume refuse to advance the old attempt and report the digest mismatch

### Requirement: Completion SHALL require machine verification
The runtime SHALL enter `completed` only after every required stage and final check has a recorded successful verification result. Reports, natural-language responses, or the absence of an error SHALL NOT be sufficient.

#### Scenario: All final checks pass
- **WHEN** the final stage report is valid and all configured final checks pass
- **THEN** the runtime records successful evidence and enters `completed`

#### Scenario: Final check fails
- **WHEN** any required final check fails
- **THEN** the runtime remains non-terminal and applies the configured retry or `needs_attention` policy
