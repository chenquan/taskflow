## MODIFIED Requirements

### Requirement: Guide safe multi-repository orchestration
The skill SHALL instruct an agent to locate the task, inspect taskflow.yaml and ownership.json when cleanup is requested, use repeated `create --repo` only to bootstrap a task that has no taskflow.yaml, edit taskflow.yaml directly when the repository topology changes, use create/delete dry-run before execute, and compose native `claude`/`codex` command lines for the user only after `create --dry-run` confirms every configured worktree is structurally ready. It MUST NOT require state, inventory, validation, dependencies, repository roles, contract owners, or an append command.

#### Scenario: Prepare multiple repositories
- **WHEN** an agent receives a Taskflow task with multiple repositories
- **THEN** it reports repository order, obtains explicit execute approval, runs create, and composes the requested native tool command only after create --dry-run confirms every repository reports `reuse`

#### Scenario: Add a repository to an existing task
- **WHEN** an agent needs another repository after taskflow.yaml already exists
- **THEN** it edits the desired repository list in taskflow.yaml, runs create --dry-run without --repo, and only runs create --execute after approval

### Requirement: Keep deterministic actions in the CLI
The skill MUST instruct agents to use taskflow for workspace, ownership-checked cleanup, worktree, and lock mutations and MUST prohibit shell-composed replacements, implicit cleanup/push/PR actions, repository append arguments on existing tasks, and nested worktree flags in composed tool commands. Delete MUST require explicit execute mode and MUST refuse resources without matching ownership records. Tool arguments requested by the user MUST be appended verbatim after the composed `--add-dir` arguments.

#### Scenario: User requests execution
- **WHEN** a user approves a prepared create plan
- **THEN** the agent invokes create execute, reports its machine-readable result, and presents the composed native tool command only when every worktree reports `reuse`
