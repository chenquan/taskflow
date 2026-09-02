## MODIFIED Requirements

### Requirement: Guide safe multi-repository orchestration
The skill SHALL instruct an agent to locate the task, inspect taskflow.yaml and ownership.json when cleanup is requested, use repeated `create --repo` and `--local` only to bootstrap a task that has no taskflow.yaml, edit taskflow.yaml directly when the repository topology or overlay paths change, use create/delete dry-run before execute, review every selected overlay path, and compose the native AI command only after every configured worktree and pending overlay is ready. It MUST NOT require state, inventory, validation, dependencies, repository roles, contract owners, or an append command.

#### Scenario: Prepare multiple repositories with local overlays
- **WHEN** an agent receives a Taskflow task with multiple repositories and explicit local overlay paths
- **THEN** it reports repository order, selected files and sizes, obtains explicit execute approval, runs create, and composes the native AI command only after live worktree and overlay checks succeed

#### Scenario: Add an overlay to an existing task
- **WHEN** an agent needs local files for a repository after taskflow.yaml already exists
- **THEN** it edits the repository's local.paths directly, runs create --dry-run without --repo or --local, and only runs create --execute after approval

### Requirement: Keep deterministic actions in the CLI
The skill MUST instruct agents to use Taskflow for workspace, overlay, ownership-checked cleanup, worktree, and lock mutations and MUST prohibit shell-composed copy replacements, implicit cleanup/push/PR actions, repository or local bootstrap arguments on existing tasks, and unsafe target overwrites. Delete MUST require explicit execute mode and MUST refuse resources without matching ownership records. Arguments explicitly supplied after `--` MUST be passed through without Taskflow policy interpretation.

#### Scenario: User requests execution
- **WHEN** a user approves a prepared create plan containing local overlay actions
- **THEN** the agent invokes create execute, reports its machine-readable worktree and overlay result, and recommends the native tool command only when every required action is complete
