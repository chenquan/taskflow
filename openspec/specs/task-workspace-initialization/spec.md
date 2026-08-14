## Purpose

Define safe, idempotent creation of a task worktree workspace.
## Requirements
### Requirement: Initialize a task workspace from explicit local repositories
The CLI SHALL provide `taskflow create <task-id>` with one or more unique `--repo <name>=<path>` values for a new task. Repository order MUST be preserved and the first repository MUST be the primary launch worktree. Execute mode MUST persist only `taskflow.yaml`; it MUST NOT create state, inventory, validation reports, branches, commits, or worktrees until the create preflight succeeds.

#### Scenario: Create three valid repositories
- **WHEN** a user creates a new task with three existing local Git repositories in a supplied order
- **THEN** dry-run reports the configuration and planned worktrees without mutation, and execute writes taskflow.yaml and creates the worktrees in that order

#### Scenario: Reject a non-Git repository
- **WHEN** an explicit repository path is not an existing non-bare Git worktree
- **THEN** create returns a configuration or environment error and does not create a taskflow.yaml or partial worktree

#### Scenario: Reject removed lifecycle flags
- **WHEN** a user invokes the removed `init` or `start` command
- **THEN** the CLI reports that the command is unavailable and instructs the user to use create

### Requirement: Initialization is idempotent and non-overwriting
Create MUST treat an existing taskflow.yaml with equivalent normalized configuration as a successful no-op for configuration and MUST reject duplicate or conflicting repository declarations rather than overwriting existing configuration. Appending a new repository MUST preserve all existing declarations and order.

#### Scenario: Repeat equivalent create
- **WHEN** a user repeats create with the same normalized task and repository inputs
- **THEN** the command succeeds, reuses existing worktrees, and does not rewrite taskflow.yaml unnecessarily

#### Scenario: Detect a conflicting declaration
- **WHEN** a create request changes the source, branch, or worktree of an existing repository name
- **THEN** create returns a configuration conflict and preserves taskflow.yaml and existing Git state

### Requirement: Initialization serializes mutations and persists atomically
Execute-mode create MUST hold a task-scoped exclusive lock while changing taskflow.yaml or creating worktrees, MUST write taskflow.yaml through an atomic same-directory replacement, and MUST complete all repository preflight before the first configuration or Git mutation. A lock conflict MUST leave taskflow.yaml and worktrees unchanged.

#### Scenario: Concurrent create attempts
- **WHEN** another process holds the task lock
- **THEN** create exits with the lock-conflict result and does not write taskflow.yaml or mutate a worktree

#### Scenario: Git creation fails after configuration persistence
- **WHEN** taskflow.yaml is persisted and a later worktree creation fails
- **THEN** the task retains the complete desired configuration, reports the failed repository, and a later create can retry from live Git state
