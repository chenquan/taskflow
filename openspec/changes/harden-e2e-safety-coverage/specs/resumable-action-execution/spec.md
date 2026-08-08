## MODIFIED Requirements

### Requirement: Persist action outcomes and resume safely
Start MUST acquire the task lock, persist phase and per-repository action outcomes atomically after each completed action, and stop subsequent actions after an error. A later start MUST derive completion from actual Git/filesystem state and may resume unfinished actions without claiming cross-repository rollback.

#### Scenario: Repeated start reuses a worktree
- **WHEN** start is run again after a worktree was created successfully
- **THEN** the second invocation reuses the matching worktree without another worktree-add action or OpenSpec action

#### Scenario: Concurrent start is rejected
- **WHEN** another start process or source-branch lock holds the required lock
- **THEN** start returns a lock conflict without changing worktrees or state
