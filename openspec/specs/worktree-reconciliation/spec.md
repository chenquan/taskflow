# worktree-reconciliation Specification

## Purpose
TBD - created by archiving change minimal-worktree-cli. Update Purpose after archive.
## Requirements
### Requirement: Reconcile declared repositories from live Git facts
The CLI SHALL provide `taskflow create <task-id> [--repo <name>=<absolute-path>]... [--dry-run|--execute]`. `taskflow.yaml` SHALL be the only persisted desired configuration; the CLI MUST derive completion and conflicts from live Git source and worktree inspection rather than a state journal, phase, action outcome, digest, inventory, or validation report.

#### Scenario: Create a new task in dry-run mode
- **WHEN** the user supplies one or more valid repositories for a task that has no taskflow.yaml and runs `create --dry-run`
- **THEN** the command reports the resolved repository configuration and planned create/reuse actions without creating the task directory, taskflow.yaml, lock directory, worktree, branch, or other Git state

#### Scenario: Reconcile an existing task without repository arguments
- **WHEN** taskflow.yaml exists and the user runs `create <task-id> --execute` without `--repo`
- **THEN** the command reconciles every declared repository from the current Git facts

#### Scenario: Append a new repository through create
- **WHEN** an existing task receives a new unique `--repo name=path` declaration
- **THEN** create appends it in declaration order, preserves every existing repository, and plans or creates only the new missing worktree

### Requirement: Reconcile worktrees idempotently and safely
Create SHALL classify each repository as `create` when its configured target is absent, or `reuse` when a registered worktree has the configured target path, source common directory, and branch. It MUST reject an existing mismatched target or occupied branch before mutation and MUST never delete, move, reset, or overwrite an existing user path.

#### Scenario: Reuse a matching worktree
- **WHEN** create is run after a matching worktree already exists
- **THEN** create succeeds with a reuse result and does not invoke `git worktree add`

#### Scenario: Reject a mismatched target
- **WHEN** the configured target exists but is not the expected worktree from the expected source and branch
- **THEN** create returns a deterministic worktree conflict before writing taskflow.yaml or changing Git state

#### Scenario: Retry after partial creation
- **WHEN** one repository worktree was created before a later repository action failed and the user reruns create after fixing the fault
- **THEN** create reuses the existing matching worktree and creates only missing worktrees without requiring persisted action state

### Requirement: Execute only after complete preflight
Execute-mode create MUST acquire the task lock and all required source/branch locks, inspect every source, base ref, branch occupancy, target path, and target identity before writing taskflow.yaml or invoking a mutating Git command. A source or branch lock MUST be acquired in deterministic order and released when create returns.

#### Scenario: Preflight blocks before mutation
- **WHEN** any repository has an unavailable base, invalid source, branch conflict, or mismatched target
- **THEN** create returns the specific diagnostic and leaves taskflow.yaml and all Git worktrees unchanged

#### Scenario: Concurrent source branch creation
- **WHEN** another task holds the same canonical source common directory and branch lock
- **THEN** create returns `SOURCE_BRANCH_LOCKED` with the conflict exit code before task configuration or Git mutation

### Requirement: Use a state-free open readiness gate
Open MUST load taskflow.yaml and verify every configured target's live worktree identity before launching Codex or Claude. It MUST NOT require or read `.taskflow/state.json`, validation reports, inventory, or a lifecycle phase. A structurally matching dirty worktree SHALL remain launchable.

#### Scenario: Open a ready workspace
- **WHEN** every configured target matches its source common directory and branch
- **THEN** open launches the selected CLI using the first repository worktree as cwd

#### Scenario: Reject an incomplete workspace
- **WHEN** a configured worktree is missing or has a different source or branch
- **THEN** open returns a structured worktree diagnostic and does not launch a child process

#### Scenario: Open a manually matching worktree
- **WHEN** a user-created worktree structurally matches the configured source, target, and branch
- **THEN** open accepts it without requiring a Taskflow ownership marker
