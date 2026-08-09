## Purpose

Define the safe multi-repository orchestration guidance shared by development agents.

## Requirements

### Requirement: Guide safe multi-repository orchestration
The skill SHALL instruct an agent to locate the task, inspect configuration, inventory, state, and validation facts, use dry-run before execute, and use status/validate after implementation.

#### Scenario: Infer repository roles
- **WHEN** an agent receives a task with multiple repositories
- **THEN** it presents proposed roles, dependencies, and contract owner, then obtains confirmation and explicit execute approval before start execute

### Requirement: Keep deterministic actions in the CLI
The skill MUST instruct agents to use `taskflow` for task-workspace, worktree, fetch, and tool-launch mutations and MUST prohibit shell composition, permission bypass flags, and implicit archive/cleanup/push/PR actions.

#### Scenario: User requests execution
- **WHEN** a user approves a prepared start plan
- **THEN** the agent invokes the CLI execute command and reports its machine-readable result
