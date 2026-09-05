## MODIFIED Requirements

### Requirement: Initialize a task workspace from explicit local repositories
The CLI SHALL provide `taskflow create <task-id>` with one or more unique `--repo <name>=<path>` values only for a new task without taskflow.yaml. Repository order MUST be preserved and the first repository MUST be the primary launch Worktree. Execute mode MUST persist taskflow.yaml and an ownership manifest for Worktrees it creates, record a pending source-copy status, and create each Worktree from the configured base followed by a complete source working-directory copy. It MUST NOT create state, inventory, validation reports, branches, commits, or Worktrees until the create preflight succeeds. Existing task configuration MUST be edited directly and reconciled through create without repository arguments.

#### Scenario: Create three valid repositories
- **WHEN** a user creates a new task with three existing local Git repositories in a supplied order
- **THEN** dry-run reports each Worktree and complete source-copy action without mutation, and execute writes taskflow.yaml, creates the Worktrees, copies each source working directory, and records ownership

#### Scenario: Reject a non-Git repository
- **WHEN** an explicit repository path is not an existing non-bare Git Worktree
- **THEN** create returns a configuration or environment error and does not create a taskflow.yaml or partial Worktree

#### Scenario: Reject repository arguments on an existing task
- **WHEN** taskflow.yaml already exists and a user invokes create with --repo
- **THEN** create returns CONFIG_EDIT_REQUIRED without changing taskflow.yaml or Git state and directs the user to edit the configuration directly

#### Scenario: Reject removed lifecycle flags
- **WHEN** a user invokes the removed init or start command
- **THEN** the CLI reports that the command is unavailable and instructs the user to use create

### Requirement: Initialization is idempotent and non-overwriting
Create MUST treat an existing taskflow.yaml as the complete desired configuration, MUST not rewrite it during reconciliation, and MUST reject repository arguments for that existing task with CONFIG_EDIT_REQUIRED. Direct edits to taskflow.yaml MUST be validated as a complete configuration before any Git mutation. Removing a repository declaration MUST NOT delete its existing Worktree. A matching Worktree with a complete source copy MUST NOT be refreshed from the source.

#### Scenario: Repeat equivalent create
- **WHEN** a user repeats create without repository arguments for the same normalized task configuration
- **THEN** the command succeeds, reuses existing Worktrees and complete source copies, and does not rewrite taskflow.yaml

#### Scenario: Detect a conflicting declaration
- **WHEN** a directly edited taskflow.yaml changes the source, branch, or Worktree of an existing repository and the live target conflicts
- **THEN** create returns a configuration or Worktree conflict and preserves taskflow.yaml and existing Git state

#### Scenario: Preserve an unlisted Worktree
- **WHEN** a repository declaration is removed from taskflow.yaml and create is rerun
- **THEN** create does not delete, move, reset, or overwrite the previously existing Worktree

### Requirement: Initialization serializes mutations and persists atomically
Execute-mode create MUST hold a task-scoped exclusive lock while creating an initial taskflow.yaml, registering Worktrees, or copying source directories, MUST write an initial taskflow.yaml through an atomic same-directory replacement, and MUST complete all repository preflight before the first configuration, Git, or target-file mutation. A lock conflict MUST leave taskflow.yaml and Worktrees unchanged.

#### Scenario: Concurrent create attempts
- **WHEN** another process holds the task lock
- **THEN** the competing command exits with the lock-conflict result and does not write taskflow.yaml or mutate a Worktree

#### Scenario: Git or source copy fails after configuration persistence
- **WHEN** an initial taskflow.yaml is persisted and a later Worktree registration or source copy fails
- **THEN** the task retains the complete desired configuration and pending ownership metadata, reports partial completion, and a later create without --repo can retry from live Git and filesystem facts
