## ADDED Requirements

### Requirement: The workflow Skill SHALL be globally installable for both Agent hosts
The bundled `taskflow-workflow` Skill SHALL be installable in the global Skill scope for both Codex and Claude using the existing Skill installation mechanism. The Skill content SHALL use the same workflow protocol and SHALL not contain task-specific state.

#### Scenario: Default global installation
- **WHEN** the user installs bundled Skills without selecting a single tool
- **THEN** the workflow Skill is installed for both Codex and Claude global Skill targets

#### Scenario: Selective global installation
- **WHEN** the user selects only Codex or only Claude
- **THEN** the workflow Skill is installed only for that selected global target and reports the target in machine-readable output

### Requirement: The workflow Skill SHALL drive one bounded iteration
When invoked in an active Codex or Claude session, the workflow Skill SHALL read the current CLI status, begin or resume only one allowed attempt, guide the Agent through one bounded unit of work, record a checkpoint, and request machine verification. It MUST NOT recursively launch Codex or Claude.

#### Scenario: Active workflow iteration
- **WHEN** the current state is eligible to run and the user invokes the workflow Skill
- **THEN** the Skill begins one attempt, provides the current stage objective, and requires checkpoint and verification before another iteration

#### Scenario: Skill would recursively launch an Agent
- **WHEN** the workflow Skill is executing inside a Codex or Claude session
- **THEN** the Skill calls Taskflow and repository commands only and does not launch a nested `codex` or `claude` process

### Requirement: Loop ticks SHALL be state-gated
The Skill's loop instruction SHALL treat the host `/loop` facility as a trigger only. At the beginning of every tick it MUST query workflow state, and it MUST stop without doing work when the state is `completed`, `paused`, `awaiting_approval`, `needs_attention`, `cancelled`, or `unknown`.

#### Scenario: Loop observes completed state
- **WHEN** `/loop` triggers after the workflow has entered `completed`
- **THEN** the Skill performs no Agent work, reports completion, and stops the loop

#### Scenario: Loop observes approval state
- **WHEN** `/loop` triggers while the workflow is `awaiting_approval`
- **THEN** the Skill does not modify files or start an attempt and reports the approval required

### Requirement: The Skill SHALL use Taskflow JSON output as its control contract
The Skill SHALL call `taskflow --json workflow ...` for state, checkpoint, and verification operations and SHALL make decisions from structured status and diagnostic codes rather than parsing human-readable prose.

#### Scenario: Structured CLI success
- **WHEN** a Taskflow command returns a successful JSON result
- **THEN** the Skill uses the returned status, attempt, stage, and evidence fields to determine the next bounded action

#### Scenario: Structured CLI conflict
- **WHEN** a Taskflow command returns a lock, lease, configuration, or worktree diagnostic
- **THEN** the Skill stops and reports the diagnostic without retrying through a shell workaround

### Requirement: The workflow SHALL be resumable across sessions
The Skill SHALL be able to resume a task in a later Codex or Claude session by reading persisted state and evidence. It MUST perform the CLI recovery checks before continuing an interrupted or stale attempt.

#### Scenario: New session resumes a paused workflow
- **WHEN** a new Agent session invokes the Skill for a task in `paused` state and the user has resumed it through the CLI
- **THEN** the Skill reads the current stage and continues from the persisted state without replaying completed verification

#### Scenario: New session finds an unknown attempt
- **WHEN** a new Agent session finds state `unknown` after the prior session ended during an attempt
- **THEN** the Skill requests recovery inspection and does not automatically replay an ambiguous action
