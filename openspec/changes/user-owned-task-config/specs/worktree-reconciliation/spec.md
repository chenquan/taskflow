## MODIFIED Requirements

### Requirement: Reconcile declared repositories from live Git facts
The CLI SHALL provide `taskflow create <task-id> [--repo <name>=<absolute-path>]... [--dry-run|--execute]`. Repeated `--repo` declarations SHALL be accepted only when the task has no `taskflow.yaml`; when `taskflow.yaml` exists, any `--repo` argument MUST return `CONFIG_EDIT_REQUIRED` and MUST NOT merge or append configuration. `taskflow.yaml` SHALL be the only persisted desired configuration; the CLI MUST derive completion and conflicts from live Git source and worktree inspection rather than a state journal, phase, action outcome, digest, inventory, or validation report.

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
