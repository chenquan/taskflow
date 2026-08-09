## Purpose

Define the safe multi-repository orchestration guidance shared by development agents.

## Requirements

### Requirement: Guide safe multi-repository orchestration
The skill SHALL instruct an agent to locate the task, read inventory, use dry-run before execute, and use status/validate after implementation.

#### Scenario: Infer repository roles
- **WHEN** an agent receives a task with multiple repositories
- **THEN** it records proposed roles, dependencies, and contract owner and asks for confirmation before start execute

### Requirement: Keep deterministic actions in the CLI
The skill MUST instruct agents to use `specflow` for Git, filesystem, OpenSpec, and tool launch mutations and MUST prohibit shell composition, permission bypass flags, and implicit archive/cleanup/push/PR actions.

#### Scenario: User requests execution
- **WHEN** a user approves a prepared start plan
- **THEN** the agent invokes the CLI execute command and reports its machine-readable result
