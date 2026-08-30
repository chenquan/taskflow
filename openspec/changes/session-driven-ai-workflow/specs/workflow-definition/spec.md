## ADDED Requirements

### Requirement: Workflow configuration is separate from worktree configuration
The system SHALL load workflow behavior from a task-local `workflow.yaml` and SHALL continue to load Git worktree behavior from `taskflow.yaml`. Workflow fields MUST NOT be added implicitly to the existing worktree configuration contract.

#### Scenario: Task has only worktree configuration
- **WHEN** a task contains a valid `taskflow.yaml` but no `workflow.yaml`
- **THEN** existing worktree commands continue to operate and workflow commands report that no workflow is configured

#### Scenario: Workflow and worktree configurations coexist
- **WHEN** a task contains valid `taskflow.yaml` and `workflow.yaml` files
- **THEN** workflow commands use both files without rewriting either user-authored configuration

### Requirement: Workflow configuration SHALL be strictly validated
The system SHALL reject unsupported workflow versions, missing required fields, duplicate stage or check identifiers, unknown fields, empty stage lists, invalid task IDs, and invalid references before creating or advancing runtime state.

#### Scenario: Valid workflow configuration
- **WHEN** `workflow.yaml` contains a supported version, matching task ID, unique linear stages, valid checks, limits, and policy
- **THEN** `workflow validate` succeeds and returns the normalized configuration digest

#### Scenario: Unknown workflow field
- **WHEN** `workflow.yaml` contains a field outside the supported schema
- **THEN** validation fails with a configuration diagnostic and does not create or modify workflow runtime files

#### Scenario: Stage references an unknown check
- **WHEN** a stage references a check identifier that is not defined
- **THEN** validation fails before `workflow begin` can create an attempt

### Requirement: Workflow stages SHALL be linear and bounded
The system SHALL preserve declared stage order, SHALL execute at most one active stage and one active attempt per task, and SHALL enforce per-stage attempt limits and global iteration, duration, and cost or usage limits when configured.

#### Scenario: Stage order is preserved
- **WHEN** a workflow declares stages `understand`, `implement`, and `review` in that order
- **THEN** the runtime exposes and advances those stages in the same order

#### Scenario: Stage attempt limit is exhausted
- **WHEN** verification fails for the current stage and its configured attempt limit is exhausted
- **THEN** the workflow enters `needs_attention` and does not start another attempt automatically

### Requirement: Verification checks SHALL be declarative and constrained
Each configured check SHALL define an executable argument array, a task-root or repository-scoped working directory, and a timeout. The verifier MUST enforce the configured working-directory boundary, timeout, output limit, and environment policy, and MUST record the check result.

#### Scenario: Repository-scoped check
- **WHEN** a check declares `cwd: repo:order-service` and `argv: ["go", "test", "./..."]`
- **THEN** the verifier runs that argument vector in the configured order-service worktree and records its exit status and bounded output

#### Scenario: Invalid check working directory
- **WHEN** a check resolves outside the task root or configured worktrees
- **THEN** validation fails and the command is never executed

#### Scenario: Check timeout
- **WHEN** a check exceeds its configured timeout
- **THEN** the verifier terminates or reaps the check process, records a timeout result, and applies the workflow retry policy
