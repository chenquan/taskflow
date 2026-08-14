## MODIFIED Requirements

### Requirement: Initialize a task workspace from explicit local repositories
The CLI SHALL provide `taskflow create <task-id>` with one or more unique `--repo <name>=<path>` values only for a new task without taskflow.yaml. Repository order MUST be preserved and the first repository MUST be the primary launch worktree. Execute mode MUST persist only taskflow.yaml; it MUST NOT create state, inventory, validation reports, branches, commits, or worktrees until the create preflight succeeds. Existing task configuration MUST be edited directly and reconciled through create without repository arguments.

#### Scenario: Create three valid repositories
- **WHEN** a user creates a new task with three existing local Git repositories in a supplied order
- **THEN** dry-run reports the configuration and planned worktrees without mutation, and execute writes taskflow.yaml and creates the worktrees in that order

#### Scenario: Reject a non-Git repository
- **WHEN** an explicit repository path is not an existing non-bare Git worktree
- **THEN** create returns a configuration or environment error and does not create a taskflow.yaml or partial worktree

#### Scenario: Reject repository arguments on an existing task
- **WHEN** taskflow.yaml already exists and a user invokes create with --repo
- **THEN** create returns CONFIG_EDIT_REQUIRED without changing taskflow.yaml or Git state and directs the user to edit the configuration directly

#### Scenario: Reject removed lifecycle flags
- **WHEN** a user invokes the removed init or start command
- **THEN** the CLI reports that the command is unavailable and instructs the user to use create

### Requirement: Initialization is idempotent and non-overwriting
Create MUST treat an existing taskflow.yaml as the complete desired configuration, MUST not rewrite it during reconciliation, and MUST reject repository arguments for that existing task with `CONFIG_EDIT_REQUIRED`. Direct edits to taskflow.yaml MUST be validated as a complete configuration before any Git mutation. Removing a repository declaration MUST NOT delete its existing worktree.

#### Scenario: Repeat equivalent create
- **WHEN** a user repeats create without repository arguments for the same normalized task configuration
- **THEN** the command succeeds, reuses existing worktrees, and does not rewrite taskflow.yaml

#### Scenario: Detect a conflicting declaration
- **WHEN** a directly edited taskflow.yaml changes the source, branch, or worktree of an existing repository and the live target conflicts
- **THEN** create returns a configuration or worktree conflict and preserves taskflow.yaml and existing Git state

#### Scenario: Preserve an unlisted worktree
- **WHEN** a repository declaration is removed from taskflow.yaml and create is rerun
- **THEN** create does not delete, move, reset, or overwrite the previously existing worktree

### Requirement: Initialization serializes mutations and persists atomically
Execute-mode create MUST hold a task-scoped exclusive lock while creating an initial taskflow.yaml or creating worktrees, MUST write an initial taskflow.yaml through an atomic same-directory replacement, and MUST complete all repository preflight before the first configuration or Git mutation. A lock conflict MUST leave taskflow.yaml and worktrees unchanged.

#### Scenario: Concurrent create attempts
- **WHEN** another process holds the task lock
- **THEN** create exits with the lock-conflict result and does not write taskflow.yaml or mutate a worktree

#### Scenario: Git creation fails after configuration persistence
- **WHEN** an initial taskflow.yaml is persisted and a later worktree creation fails
- **THEN** the task retains the complete desired configuration, reports the failed repository, and a later create without --repo can retry from live Git state
