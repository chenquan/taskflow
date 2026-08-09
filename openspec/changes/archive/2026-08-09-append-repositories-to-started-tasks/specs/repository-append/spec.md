## ADDED Requirements

### Requirement: Append repositories to an existing task
The CLI SHALL provide `taskflow repo add <task-id> --repo <name>=<absolute-path> [--depends-on <repo>] [--dry-run]` that appends one repository to an already-initialized task as a metadata-only change. The command MUST validate that the repository path exists, is a Git repository, the name is unique among the task repositories, and every `--depends-on` value references an existing repository other than the appended repository. The appended repository MUST reuse the `init` defaults: `base: HEAD`, `branch: feature/<task-id>`, `worktree: worktrees/<name>`, no checks, and no dependencies unless `--depends-on` is supplied. The command MUST be append-only: it MUST NOT modify or remove existing repositories and MUST NOT change the primary repository.

#### Scenario: Append a second repository
- **WHEN** a task with one repository is started and the user appends a second valid repository with `repo add`
- **THEN** the task configuration gains the second repository with the default base, branch, and worktree, and no worktree is created until the next `start --execute`

#### Scenario: Reject a duplicate repository name
- **WHEN** the appended repository name already exists in the task
- **THEN** `repo add` returns a configuration error and leaves every task file unchanged

#### Scenario: Reject an unknown dependency
- **WHEN** `--depends-on` names a repository that is not in the task or names the appended repository itself
- **THEN** `repo add` returns a configuration error before writing any file

#### Scenario: Reject a non-Git source path
- **WHEN** the appended repository path is not a Git repository
- **THEN** `repo add` returns an environment error and leaves every task file unchanged

### Requirement: Gate append by task phase and task lock
`repo add` MUST acquire the task lock before mutating any task file and MUST only append while the persisted task phase is `initialized`, `started`, or `failed`. For any other phase, including a transient `starting` phase, `repo add` MUST return a configuration error before writing and instruct the caller to recover the in-progress start first.

#### Scenario: Append to a started task
- **WHEN** the task phase is `started` and no other process holds the task lock
- **THEN** `repo add` acquires the lock and appends the repository

#### Scenario: Reject append during a transient phase
- **WHEN** the persisted task phase is `starting`
- **THEN** `repo add` returns a configuration error naming the supported phases and leaves every task file unchanged

#### Scenario: Reject concurrent append
- **WHEN** another process holds the task lock
- **THEN** `repo add` returns a lock conflict and leaves every task file unchanged

### Requirement: Migrate configuration digest atomically
A successful `repo add` MUST update `taskflow.yaml`, `.taskflow/inventory.json`, and `.taskflow/state.json` as one atomic migration. The migration MUST preserve existing repository action outcomes and worktree references, MUST add a pending fetch (or skipped fetch when `execution.fetch` is false) and pending worktree action for the appended repository, MUST append the appended repository's Git facts to the inventory, and MUST advance the persisted configuration digest to the normalized appended configuration. If any write fails, the command MUST restore the prior contents of all three files and return an execution error, leaving existing task state and worktrees unchanged.

#### Scenario: Preserve completed work while adding pending state
- **WHEN** a started task with a completed worktree for its first repository appends a second repository
- **THEN** the first repository's completed actions remain in `state.json`, the second repository receives pending actions, and the persisted digest matches the appended configuration

#### Scenario: Roll back on write failure
- **WHEN** a write to any of the three task files fails after another has succeeded
- **THEN** `repo add` restores all three files to their prior contents and returns an execution error

### Requirement: Preview an append with dry-run
`repo add --dry-run` MUST perform the same validation as the mutating form, then report the resolved appended repository and the start actions that would create its worktree, without acquiring the task lock or writing any file.

#### Scenario: Dry-run reports the resolved repository and actions
- **WHEN** the user runs `repo add --dry-run` with a valid repository
- **THEN** the result includes the resolved repository configuration and the pending start actions for that repository, and no task file is modified
