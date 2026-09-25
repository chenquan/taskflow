## MODIFIED Requirements

### Requirement: Reconcile declared repositories from live Git facts
The CLI SHALL provide `taskflow create <task-id> [--repo <name>=<absolute-path>]... [--dry-run|--execute]`. Repeated `--repo` declarations SHALL be accepted only when the task has no `taskflow.yaml`; when `taskflow.yaml` exists, any `--repo` argument MUST return `CONFIG_EDIT_REQUIRED` and MUST NOT merge or append configuration. `taskflow.yaml` SHALL be the only persisted desired configuration; the CLI MUST derive completion and conflicts from live Git source and worktree inspection, and the only persisted derivation is the repository-level source-copy status (`pending` or `complete`) in the Taskflow ownership manifest, because an interrupted copy is indistinguishable from a complete one by inspection alone.

#### Scenario: Create a new task in dry-run mode
- **WHEN** the user supplies one or more valid repositories for a task that has no taskflow.yaml and runs `create --dry-run`
- **THEN** the command reports the resolved repository configuration and planned create/reuse actions without creating the task directory, taskflow.yaml, lock directory, worktree, branch, or other Git state

#### Scenario: Reconcile an existing task without repository arguments
- **WHEN** taskflow.yaml exists and the user runs `create <task-id> --execute` without `--repo`
- **THEN** the command reconciles every declared repository from the current Git facts without rewriting taskflow.yaml

#### Scenario: Reject repository arguments for an existing task
- **WHEN** taskflow.yaml exists and the user runs `create <task-id> --repo <name>=<path>`
- **THEN** create returns `CONFIG_EDIT_REQUIRED` before configuration or Git mutation and instructs the user or agent to edit taskflow.yaml directly before rerunning create without --repo

#### Scenario: Reconcile a directly edited configuration
- **WHEN** a user or agent adds a valid repository declaration to an existing taskflow.yaml and runs create
- **THEN** create validates the complete configuration and plans or creates only the missing worktree while preserving all existing declarations and worktrees

### Requirement: Reconcile worktrees idempotently and safely
Create SHALL classify each repository as `create` when its configured target is absent, or `reuse` when a registered worktree has the configured target path, source common directory, and branch. It SHALL classify each repository's source-copy action as `copy` for a missing target, `repair` for a matching Taskflow-owned target whose recorded copy is pending, or `reuse` for a complete copy or a matching worktree without a Taskflow copy record, which MUST never be populated implicitly. It MUST reject an existing mismatched target or occupied branch before mutation and MUST never delete, move, reset, or overwrite an existing user path.

#### Scenario: Reuse a matching worktree
- **WHEN** create is run after a matching worktree already exists
- **THEN** create succeeds with a reuse result and does not invoke `git worktree add`

#### Scenario: Never populate an unowned matching worktree
- **WHEN** a registered worktree matches the configuration but has no Taskflow source-copy ownership record
- **THEN** create reports the source copy as reused with a reason and copies nothing into the target

#### Scenario: Reject a mismatched target
- **WHEN** the configured target exists but is not the expected worktree from the expected source and branch
- **THEN** create returns a deterministic worktree conflict before writing taskflow.yaml or changing Git state

#### Scenario: Retry after partial creation
- **WHEN** one repository worktree was created before a later repository action failed and the user reruns create after fixing the fault
- **THEN** create reuses the existing matching worktree and creates only missing worktrees without requiring persisted action state
