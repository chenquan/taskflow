# taskflow-multirepo-skill Specification

## Purpose

Define the safe multi-repository orchestration guidance shared by development agents.

## Requirements

### Requirement: Guide safe multi-repository orchestration
The skill SHALL instruct an agent to locate the task, inspect taskflow.yaml and ownership.json when cleanup is requested, use repeated `create --repo` only to bootstrap a task that has no taskflow.yaml, edit taskflow.yaml directly when the repository topology changes, use create/delete dry-run before execute, and use open only after every configured worktree is structurally ready. It MUST NOT require state, inventory, validation, dependencies, repository roles, contract owners, or an append command.

#### Scenario: Prepare multiple repositories
- **WHEN** an agent receives a Taskflow task with multiple repositories
- **THEN** it reports repository order, obtains explicit execute approval, runs create, and opens the requested built-in tool only after live worktree checks succeed

#### Scenario: Add a repository to an existing task
- **WHEN** an agent needs another repository after taskflow.yaml already exists
- **THEN** it edits the desired repository list in taskflow.yaml, runs create --dry-run without --repo, and only runs create --execute after approval

### Requirement: Keep deterministic actions in the CLI
The skill MUST instruct agents to use taskflow for workspace, ownership-checked cleanup, worktree, lock, and built-in tool-launch mutations and MUST prohibit shell-composed replacements, implicit cleanup/push/PR actions, repository append arguments on existing tasks, and nested worktree launch flags. Delete MUST require explicit execute mode and MUST refuse resources without matching ownership records. Arguments explicitly supplied after -- MUST be passed through without Taskflow policy interpretation.

#### Scenario: User requests execution
- **WHEN** a user approves a prepared create plan
- **THEN** the agent invokes create execute, reports its machine-readable result, and recommends open only when every worktree is ready
