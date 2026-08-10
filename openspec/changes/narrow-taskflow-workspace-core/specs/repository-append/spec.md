## MODIFIED Requirements

### Requirement: Append repositories to an existing task
The CLI SHALL provide `taskflow repo add <task-id> --repo <name>=<absolute-path> [--depends-on <repo>] [--dry-run]` that appends one repository to an already-initialized task as a metadata-only change. The command MUST validate that the repository path exists, is a Git repository, the name is unique among the task repositories, and every `--depends-on` value references an existing repository other than the appended repository. The appended repository MUST reuse the `init` defaults: `base: HEAD`, `branch: feature/<task-id>`, `worktree: worktrees/<name>`, no checks, and no dependencies unless `--depends-on` is supplied. The command MUST be append-only: it MUST NOT modify, remove, or reorder existing repositories, so the first repository remains the primary launch worktree.

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

### Requirement: Migrate configuration digest atomically
A successful `repo add` MUST update `taskflow.yaml` and `.taskflow/state.json` as one atomic migration. The migration MUST preserve existing repository action outcomes and worktree references, MUST add a pending fetch (or skipped fetch when `execution.fetch` is false) and pending worktree action for the appended repository, and MUST advance the persisted configuration digest to the normalized appended configuration. If either write fails, the command MUST restore both files and return an execution error, leaving existing task state, worktrees, and any legacy inventory file unchanged.

#### Scenario: Preserve completed work while adding pending state
- **WHEN** a started task with a completed worktree for its first repository appends a second repository
- **THEN** the first repository's completed actions remain in `state.json`, the second repository receives pending actions, and the persisted digest matches the appended configuration

#### Scenario: Roll back on write failure
- **WHEN** writing either task file fails after the other has succeeded
- **THEN** `repo add` restores both files and returns an execution error

#### Scenario: Preserve a legacy inventory file
- **WHEN** a schema-v2 workspace contains an unrelated legacy `.taskflow/inventory.json`
- **THEN** `repo add` neither reads, updates, nor removes that file
