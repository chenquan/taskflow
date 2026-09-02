## MODIFIED Requirements

### Requirement: Reconcile declared repositories from live Git facts
The CLI SHALL provide `taskflow create <task-id> [--repo <name>=<absolute-path>]... [--local <name>=<source-relative-path>]... [--dry-run|--execute]`. Repeated `--repo` and `--local` declarations SHALL be accepted only when the task has no taskflow.yaml; when taskflow.yaml exists, any bootstrap argument MUST return `CONFIG_EDIT_REQUIRED` and MUST NOT merge or append configuration. `taskflow.yaml` SHALL remain the only persisted desired configuration, while optional overlay metadata in ownership.json records materialization recovery for Taskflow-created worktrees. The CLI MUST derive worktree and overlay completion and conflicts from live Git, filesystem, and ownership facts rather than a general task state journal, phase, action outcome, digest, inventory, or validation report.

#### Scenario: Create a new task in dry-run mode
- **WHEN** the user supplies valid repositories and optional local overlay declarations for a task without taskflow.yaml and runs `create --dry-run`
- **THEN** the command reports the resolved repository configuration, worktree actions, and selected overlay files without creating the task directory, configuration, ownership metadata, lock directory, worktree, branch, or target file

#### Scenario: Reconcile an existing task without bootstrap arguments
- **WHEN** taskflow.yaml exists and the user runs `create <task-id> --execute` without `--repo` or `--local`
- **THEN** the command reconciles every declared repository and any pending owned overlay from current Git and filesystem facts without rewriting taskflow.yaml

#### Scenario: Reject bootstrap arguments for an existing task
- **WHEN** taskflow.yaml exists and the user runs create with `--repo` or `--local`
- **THEN** create returns `CONFIG_EDIT_REQUIRED` before configuration, ownership, overlay, or Git mutation and instructs the user or agent to edit taskflow.yaml directly before rerunning create without bootstrap arguments

#### Scenario: Reconcile a directly edited configuration
- **WHEN** a user or agent adds a valid repository and local overlay declaration to an existing taskflow.yaml and the target worktree is missing
- **THEN** create validates the complete configuration and plans or creates only the missing worktree and its overlay while preserving all existing declarations and worktrees

### Requirement: Reconcile worktrees idempotently and safely
Create SHALL classify each repository as `create` when its configured target is absent, or `reuse` when a registered or manually matching worktree has the configured target path, source common directory, and branch. It MUST reject an existing mismatched target or occupied branch before mutation and MUST never delete, move, reset, or overwrite an existing user path. Overlay materialization SHALL be a separate action: it is performed for a newly created Taskflow worktree, repaired only from a pending Taskflow ownership snapshot, and skipped for a matching manually managed worktree.

#### Scenario: Reuse a matching worktree without overlay refresh
- **WHEN** create is run after a matching worktree has a complete overlay or has no overlay declaration
- **THEN** create succeeds with reuse actions and does not invoke `git worktree add` or overwrite local files

#### Scenario: Reject a mismatched target
- **WHEN** the configured target exists but is not the expected worktree from the expected source and branch
- **THEN** create returns a deterministic worktree conflict before writing taskflow.yaml, ownership metadata, overlay files, or changing Git state

#### Scenario: Retry after partial creation
- **WHEN** one repository worktree or overlay was created before a later action failed and the user reruns create after fixing the fault
- **THEN** create reuses the existing matching worktree, repairs only the pending overlay files, and creates only missing repositories without overwriting existing files

#### Scenario: Preserve a matching manual worktree
- **WHEN** a user-created worktree structurally matches the configured target and branch but has no Taskflow ownership entry
- **THEN** create reports worktree reuse and does not materialize the configured local overlay into that worktree

### Requirement: Execute only after complete preflight
Execute-mode create MUST acquire the task lock and all required source/branch locks, inspect every source, base ref, branch occupancy, target path, target identity, selected overlay path, source file type, source snapshot hash, and base-tree collision before writing an initial taskflow.yaml, ownership metadata, or invoking a mutating Git or filesystem command. A source or branch lock MUST be acquired in deterministic order and released when create returns.

#### Scenario: Preflight blocks before mutation
- **WHEN** any repository has an unavailable base, invalid source, branch conflict, mismatched target, missing overlay path, unsafe overlay file, or base-tree collision
- **THEN** create returns the specific diagnostic and leaves taskflow.yaml, ownership metadata, all Git worktrees, and overlay files unchanged

#### Scenario: Concurrent source branch creation
- **WHEN** another task holds the same canonical source common directory and branch lock
- **THEN** create returns `SOURCE_BRANCH_LOCKED` with the conflict exit code before task configuration, ownership, overlay, or Git mutation
