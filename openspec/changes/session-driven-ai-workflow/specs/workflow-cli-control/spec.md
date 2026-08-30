## ADDED Requirements

### Requirement: The CLI SHALL expose workflow lifecycle operations
The CLI SHALL provide `workflow validate`, `workflow status`, `workflow begin`, `workflow checkpoint`, `workflow verify`, `workflow pause`, `workflow resume`, `workflow approve`, and `workflow cancel` commands under the existing Taskflow root command. The commands SHALL accept the existing task selection and JSON output conventions.

#### Scenario: Inspect workflow status
- **WHEN** the user runs `taskflow --json workflow status <task-id>`
- **THEN** the CLI returns the current state, stage, iteration, lease/approval summary, last verification, and diagnostic warnings without changing files

#### Scenario: Unknown workflow command argument
- **WHEN** a workflow command receives an invalid task ID, unsupported option, or missing required argument
- **THEN** the CLI returns a structured configuration or argument diagnostic without mutating runtime or Git state

### Requirement: Workflow validation SHALL precede mutation
`workflow begin` and every command that advances or executes a workflow SHALL validate the task configuration, workflow configuration, worktree readiness, configuration digest, current state, and lease requirements before writing runtime state or running a check.

#### Scenario: Worktree is not ready
- **WHEN** a workflow begins while a configured worktree is missing or has mismatched Git identity
- **THEN** the CLI returns a worktree diagnostic and does not start an attempt or execute verification commands

#### Scenario: Valid workflow begins
- **WHEN** all configuration, worktree, state, and lease preconditions pass
- **THEN** `workflow begin` creates one attempt, persists its metadata, and returns the stage objective and attempt token

### Requirement: Checkpoint input SHALL be schema-validated
`workflow checkpoint` SHALL accept a structured report for the active attempt and SHALL validate status, attempt identity, changed paths, commands, risks, and summary fields before persisting the checkpoint. Invalid reports SHALL not advance workflow state.

#### Scenario: Valid ready checkpoint
- **WHEN** the active Agent submits a valid `ready` checkpoint for the current attempt
- **THEN** the CLI persists the report and makes the attempt eligible for `workflow verify`

#### Scenario: Invalid or foreign checkpoint
- **WHEN** a checkpoint is malformed or references another task, stage, or attempt
- **THEN** the CLI returns a checkpoint diagnostic and leaves the active state and event history unchanged

### Requirement: Verification SHALL execute only configured checks
`workflow verify` SHALL execute the checks referenced by the current stage using the configured argv, bounded working directory, timeout, environment policy, and output limit. It SHALL persist individual results and a stage-level result.

#### Scenario: Verification succeeds
- **WHEN** every required check for the current stage exits with success
- **THEN** the CLI records successful check evidence and advances the workflow according to the stage order

#### Scenario: Verification fails
- **WHEN** one or more required checks fail or time out
- **THEN** the CLI records the failure evidence and either creates an eligible retry or enters `needs_attention` according to the configured limits

### Requirement: Approval commands SHALL record decisions without bypassing policy
`workflow approve` SHALL record a human approval or rejection for a named approval request. Approval SHALL NOT authorize an action absent from the workflow policy, alter the worktree configuration, or bypass task, lease, configuration, or verification checks.

#### Scenario: Approval is granted
- **WHEN** a valid pending approval request is approved by the user
- **THEN** the CLI records the decision and changes the workflow to the next policy-allowed state without executing an external side effect itself

#### Scenario: Unknown approval request
- **WHEN** the user approves an expired, completed, rejected, or unknown approval ID
- **THEN** the CLI returns an approval conflict and leaves workflow state unchanged

### Requirement: Pause, resume, and cancel SHALL be explicit and recoverable
`workflow pause`, `workflow resume`, and `workflow cancel` SHALL be idempotent, SHALL preserve prior evidence, and SHALL not delete or reset worktrees. Resume SHALL require valid configuration and safe lease recovery; cancel SHALL leave the task available for inspection and separate user-authorized cleanup.

#### Scenario: User pauses an active workflow
- **WHEN** the user pauses a workflow with an active or retryable attempt
- **THEN** the CLI records the reason, releases or expires the active lease safely, and enters `paused`

#### Scenario: User resumes a paused workflow
- **WHEN** the workflow configuration and worktrees remain valid and the user resumes the task
- **THEN** the CLI creates a new valid execution context or reactivates the eligible stage without deleting previous attempts

#### Scenario: User cancels a workflow
- **WHEN** the user cancels a workflow in a non-terminal state
- **THEN** the CLI records cancellation, stops future workflow advancement, and leaves worktrees and evidence untouched

### Requirement: Workflow commands SHALL not launch Agents or perform release-side effects
Workflow CLI commands SHALL not start Codex or Claude, commit or push Git changes, create or merge pull requests, release or deploy artifacts, delete worktrees, or write external systems. They SHALL provide status and evidence for the active Skill session to act on.

#### Scenario: Skill calls workflow CLI
- **WHEN** an active Agent Skill invokes `workflow begin`, `checkpoint`, or `verify`
- **THEN** the CLI performs only the declared local state or verification operation and returns structured evidence

#### Scenario: Delete is requested through workflow
- **WHEN** a workflow attempts to delete a worktree or task resource
- **THEN** the workflow command rejects the action and directs the user to the existing separately authorized delete flow
