## Purpose

Define safe, idempotent initialization of a task control workspace.

## Requirements

### Requirement: Initialize a task workspace from explicit local repositories
The CLI SHALL provide `taskflow init <task-id>` with an explicit tasks root and one or more unique `--repo <name>=<path>` values. An optional primary repository selection may override the first repository. It MUST create a task control directory containing `taskflow.yaml`, `.taskflow/inventory.json`, and `.taskflow/state.json` without creating branches, worktrees, commits, or fetching remotes.

#### Scenario: Initialize three valid repositories
- **WHEN** a user initializes a new task with three existing local Git repositories and a valid primary repository
- **THEN** the CLI writes the task metadata and reports success without modifying Git state in any source repository

#### Scenario: Reject a non-Git repository
- **WHEN** an explicit repository path is not an existing Git work tree
- **THEN** the CLI returns a configuration error and does not create a partial task workspace

### Requirement: Initialization is idempotent and non-overwriting
The CLI MUST treat a pre-existing workspace with equivalent normalized configuration as successful without rewriting user-authored files. It MUST reject a conflicting configuration or unmanaged existing files rather than overwriting them.

#### Scenario: Repeat equivalent initialization
- **WHEN** a user repeats `init` with the same normalized task and repository inputs
- **THEN** the CLI succeeds and reports that the workspace is already initialized

#### Scenario: Detect conflicting initialization
- **WHEN** a user initializes an existing task directory with a different repository mapping
- **THEN** the CLI returns a configuration conflict and preserves the existing configuration

### Requirement: Initialization serializes mutations and persists atomically
The CLI MUST hold a task-scoped exclusive lock while mutating a task workspace and MUST write machine state through an atomic same-directory replacement. A lock conflict MUST leave the existing workspace unchanged.

#### Scenario: Concurrent initialization attempts
- **WHEN** another process holds the task lock
- **THEN** `init` exits with the lock-conflict result and does not write task metadata
