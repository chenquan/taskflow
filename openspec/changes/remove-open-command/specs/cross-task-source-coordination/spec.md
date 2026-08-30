## MODIFIED Requirements

### Requirement: Coordinate execute-mode source branches across tasks
Before execute-mode create mutates taskflow.yaml or creates a worktree, the CLI MUST acquire a local exclusive lock for every configured `(canonical Git common directory, branch)` pair. Locks MUST be stored beneath that common directory, acquired in deterministic sorted order, and released after create returns. A held lock MUST return exit code 5 with a structured `SOURCE_BRANCH_LOCKED` diagnostic; a lock-storage failure MUST return an environment failure. Read-only dry-run commands and creates for different branches MUST NOT contend.

#### Scenario: Competing tasks use the same source branch
- **WHEN** one task holds an execute-mode lock for a source repository branch
- **THEN** another task creating the same source branch exits with `SOURCE_BRANCH_LOCKED` before taskflow.yaml or Git mutation

#### Scenario: Different source branches proceed independently
- **WHEN** two tasks target different branches in the same source repository
- **THEN** each task can acquire its own source-branch lock
