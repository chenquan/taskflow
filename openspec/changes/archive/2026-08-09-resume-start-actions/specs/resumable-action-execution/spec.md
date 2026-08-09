## MODIFIED Requirements

### Requirement: Persist action outcomes and resume safely
Start MUST acquire the task lock, persist phase and per-repository action outcomes atomically after each completed action, and stop subsequent actions after an error. A later start MUST load compatible persisted state, reconcile completion with actual Git/filesystem state, and resume unfinished actions without claiming cross-repository rollback. A start MUST reject malformed state or a state whose configuration digest differs from the normalized task configuration before mutation.

#### Scenario: Failure resumes from the first incomplete action
- **WHEN** start fails after a prior repository worktree was created
- **THEN** state records the completed worktree and failure, and a later start reuses the prior worktree and continues from the first incomplete action

#### Scenario: Completed fetch is not repeated
- **WHEN** a previous start completed fetch and the configured base ref remains available
- **THEN** a later start skips fetch and proceeds without issuing another fetch mutation

#### Scenario: Missing completed worktree is recreated
- **WHEN** persisted state marks a worktree completed but the configured managed worktree no longer exists and no conflicting target is present
- **THEN** a later start recreates that worktree and persists the refreshed completed outcome

#### Scenario: Configuration change blocks resume
- **WHEN** persisted start state contains a different configuration digest from the currently loaded task
- **THEN** start returns a structured state-conflict failure before changing state, Git, or the filesystem

#### Scenario: Corrupt state blocks resume
- **WHEN** `.taskflow/state.json` is malformed or incompatible
- **THEN** start returns a structured state-incompatible failure and preserves the existing state file

#### Scenario: Concurrent start is rejected
- **WHEN** another start process holds the task lock
- **THEN** start returns a lock conflict without changing worktrees or state
