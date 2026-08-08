## Purpose

Define recoverable start execution with durable action outcomes.

## Requirements

### Requirement: Persist action outcomes and resume safely
Start MUST acquire the task lock, persist phase and per-repository action outcomes atomically after each completed action, and stop subsequent actions after an error. A later start MUST derive completion from actual Git/filesystem state and may resume unfinished actions without claiming cross-repository rollback.

#### Scenario: Failure resumes from the first incomplete action
- **WHEN** an OpenSpec action fails after a prior repository worktree was created
- **THEN** state records the completed worktree and failure, and a later start reuses the prior worktree before continuing

#### Scenario: Concurrent start is rejected
- **WHEN** another start process holds the task lock
- **THEN** start returns a lock conflict without changing worktrees or state
