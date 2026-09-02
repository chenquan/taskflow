## MODIFIED Requirements

### Requirement: Initialize a task workspace from explicit local repositories
The CLI SHALL provide `taskflow create <task-id>` with one or more unique `--repo <name>=<path>` values and optional repeatable `--local <name>=<source-relative-path>` values only for a new task without taskflow.yaml. Repository order MUST be preserved and the first repository MUST be the primary launch worktree. Execute mode MUST persist taskflow.yaml and an ownership manifest for worktrees and pending overlay snapshots it creates; it MUST NOT create a general task state journal, inventory, validation reports, branches, commits, or worktrees until complete preflight succeeds. Existing task configuration MUST be edited directly and reconciled through create without repository or local bootstrap arguments.

#### Scenario: Create repositories with an overlay declaration
- **WHEN** a user creates a new task with valid local repositories and a `--local` path for one repository
- **THEN** dry-run reports the repository configuration and overlay plan without mutation, and execute persists the path and creates the worktree with the selected file

#### Scenario: Reject a non-Git repository
- **WHEN** an explicit repository path is not an existing non-bare Git worktree
- **THEN** create returns a configuration or environment error and does not create a taskflow.yaml, ownership manifest, partial worktree, or overlay file

#### Scenario: Reject repository or local arguments on an existing task
- **WHEN** taskflow.yaml already exists and a user invokes create with `--repo` or `--local`
- **THEN** create returns CONFIG_EDIT_REQUIRED without changing taskflow.yaml, ownership metadata, overlay files, or Git state and directs the user to edit the configuration directly

#### Scenario: Reject removed lifecycle flags
- **WHEN** a user invokes the removed init or start command
- **THEN** the CLI reports that the command is unavailable and instructs the user to use create

### Requirement: Initialization is idempotent and non-overwriting
Create MUST treat an existing taskflow.yaml as the complete desired configuration, MUST not rewrite it during reconciliation, and MUST reject repository or local bootstrap arguments for that existing task with `CONFIG_EDIT_REQUIRED`. Direct edits to taskflow.yaml MUST be validated as a complete configuration before any Git or overlay mutation. A matching worktree with a complete overlay MUST be reused without refreshing its files, and removing a repository declaration MUST NOT delete its existing worktree or copied files.

#### Scenario: Repeat equivalent create
- **WHEN** a user repeats create without bootstrap arguments for the same normalized task configuration
- **THEN** the command succeeds, reuses existing worktrees and completed overlays, and does not rewrite taskflow.yaml or overlay files

#### Scenario: Detect a conflicting declaration
- **WHEN** a directly edited taskflow.yaml changes the source, branch, worktree, or overlay destination relationship of an existing repository and the live target conflicts
- **THEN** create returns a configuration or worktree/overlay conflict and preserves taskflow.yaml, ownership metadata, existing worktrees, and existing files

#### Scenario: Preserve an unlisted worktree
- **WHEN** a repository declaration is removed from taskflow.yaml and create is rerun
- **THEN** create does not delete, move, reset, or overwrite the previously existing worktree or its local files

#### Scenario: Repair a pending overlay
- **WHEN** an owned worktree has a pending overlay snapshot from an interrupted create
- **THEN** create repairs the pending snapshot without recreating the worktree or refreshing any completed overlay

### Requirement: Initialization serializes mutations and persists atomically
Execute-mode create MUST hold a task-scoped exclusive lock while creating an initial taskflow.yaml, ownership manifest, worktree, or overlay. It MUST write taskflow.yaml and ownership metadata through atomic same-directory replacements, and MUST complete all repository and overlay preflight before the first configuration, manifest, Git, or target-file mutation. A lock conflict MUST leave taskflow.yaml, ownership metadata, worktrees, branches, and overlay files unchanged.

#### Scenario: Concurrent create attempts
- **WHEN** another process holds the task lock
- **THEN** create exits with the lock-conflict result and does not write taskflow.yaml, ownership metadata, worktrees, branches, or overlay files

#### Scenario: Git creation fails after configuration persistence
- **WHEN** an initial taskflow.yaml and pending ownership snapshot are persisted but a later worktree creation fails
- **THEN** the task retains the complete desired configuration, reports the failed repository, and a later create without bootstrap arguments can retry from live Git and pending overlay facts

#### Scenario: Overlay copying fails after worktree creation
- **WHEN** a worktree is created but a selected overlay file cannot be published
- **THEN** create reports partial completion, retains the pending ownership snapshot and created worktree, and a later create can repair the overlay without overwriting existing files
