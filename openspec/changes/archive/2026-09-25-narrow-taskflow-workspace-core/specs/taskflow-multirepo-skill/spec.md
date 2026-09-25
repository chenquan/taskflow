## MODIFIED Requirements

### Requirement: Guide safe multi-repository orchestration
The skill SHALL instruct an agent to locate the task, inspect configuration, state, current status, and historical validation facts, use dry-run before execute, use `open` only after successful start, and use status/validate after implementation. The skill MUST describe `depends_on` only as execution and validation order and MUST NOT require repository roles, contract owners, or inventory inspection.

#### Scenario: Prepare multiple repositories
- **WHEN** an agent receives a Taskflow task with multiple repositories
- **THEN** it reports repository order and configured dependencies, obtains explicit execute approval, starts the workspace, and opens the requested built-in tool only after readiness succeeds

### Requirement: Keep deterministic actions in the CLI
The skill MUST instruct agents to use `taskflow` for task-workspace, worktree, fetch, repository-append, and built-in tool-launch mutations and MUST prohibit shell-composed replacements, implicit archive/cleanup/push/PR actions, and nested worktree launch flags. Permission and model arguments explicitly supplied after `--` MUST be passed through without Taskflow policy interpretation.

#### Scenario: User requests execution
- **WHEN** a user approves a prepared start plan
- **THEN** the agent invokes the CLI execute command, reports its machine-readable result, and recommends `open` only when every managed worktree is ready
