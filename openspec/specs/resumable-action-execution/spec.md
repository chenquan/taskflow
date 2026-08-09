## Purpose

Define recoverable start execution with durable action outcomes.

## Requirements

### Requirement: Persist action outcomes and resume safely
Start MUST acquire the task lock, persist `.taskflow/state.json` phase and per-repository action outcomes atomically after each completed action, and stop subsequent actions after an error. A later start MUST derive completion from actual Git/filesystem state and may resume unfinished actions without claiming cross-repository rollback. Start MUST reject malformed state or a persisted configuration digest that differs from the normalized task configuration before mutation.

#### Scenario: Failure resumes from the first incomplete action
- **WHEN** a fetch or worktree action fails after a prior repository worktree was created
- **THEN** state records the completed worktree and failure, and a later start reuses the prior worktree before continuing

#### Scenario: Configuration change blocks resume
- **WHEN** persisted start state contains a different configuration digest from the currently loaded task
- **THEN** start returns `STATE_CONFLICT` before changing state, Git, or the filesystem

#### Scenario: Corrupt state blocks resume
- **WHEN** `.taskflow/state.json` is malformed or incompatible
- **THEN** start returns `STATE_INCOMPATIBLE` and preserves the existing state file

#### Scenario: Concurrent start is rejected
- **WHEN** another start process holds the task lock
- **THEN** start returns a lock conflict without changing worktrees or state

### Requirement: Accept authorized configuration advancement
Start MUST continue to reject a persisted configuration digest that differs from the normalized task configuration. The `repo add` migration is the only authorized way to advance that digest: after it advances the digest while preserving existing action outcomes, a subsequent `start --execute` MUST accept the matching digest and reconcile only repositories whose actions are still pending, reusing existing worktrees and creating worktrees only for appended repositories.

#### Scenario: Resume after an authorized append
- **WHEN** a started task appends a second repository via `repo add` and the user runs `start --execute`
- **THEN** start accepts the advanced digest, reuses the first repository's existing worktree, and creates only the appended repository's worktree

#### Scenario: Repeated start is idempotent after append
- **WHEN** `start --execute` runs again after the appended repository's worktree exists
- **THEN** start reports both worktrees as already complete and creates no duplicate branch or directory
