## MODIFIED Requirements

### Requirement: Persist action outcomes and resume safely
After global preflight succeeds, start MUST persist task phase and typed directory/fetch/worktree/OpenSpec action outcomes atomically after each action. Outcomes MUST be one of `pending`, `completed`, `skipped`, or `failed` with an update time and optional error. Start MUST stop subsequent actions after an execution error. A later start MUST derive completion from actual Git/filesystem/OpenSpec state, reconcile stored outcomes, and resume unfinished actions without claiming cross-repository rollback.

#### Scenario: Failure resumes from the first incomplete action
- **WHEN** an OpenSpec action fails after its repository worktree was created
- **THEN** state records the worktree action as completed and OpenSpec action as failed, and a later start reuses the prior worktree before continuing

#### Scenario: Concurrent start is rejected
- **WHEN** another start process holds the task lock
- **THEN** start returns a lock conflict without changing worktrees or state

#### Scenario: Preflight conflict preserves prior state
- **WHEN** global preflight finds a deterministic target or branch conflict
- **THEN** the existing state file remains byte-for-byte unchanged and no action outcome is recorded
