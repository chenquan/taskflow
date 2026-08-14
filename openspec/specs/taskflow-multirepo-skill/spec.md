## Purpose

Define the safe multi-repository orchestration guidance shared by development agents.
## Requirements
### Requirement: Guide safe multi-repository orchestration
The skill SHALL instruct an agent to locate the task, inspect taskflow.yaml, use create dry-run before execute, and use open only after every configured worktree is structurally ready. It MUST NOT require state, inventory, validation, dependencies, repository roles, or contract owners.

#### Scenario: Prepare multiple repositories
- **WHEN** an agent receives a Taskflow task with multiple repositories
- **THEN** it reports repository order, obtains explicit execute approval, runs create, and opens the requested built-in tool only after live worktree checks succeed

### Requirement: Keep deterministic actions in the CLI
The skill MUST instruct agents to use taskflow for workspace, worktree, lock, and built-in tool-launch mutations and MUST prohibit shell-composed replacements, implicit cleanup/push/PR actions, and nested worktree launch flags. Arguments explicitly supplied after `--` MUST be passed through without Taskflow policy interpretation.

#### Scenario: User requests execution
- **WHEN** a user approves a prepared create plan
- **THEN** the agent invokes create execute, reports its machine-readable result, and recommends open only when every worktree is ready
