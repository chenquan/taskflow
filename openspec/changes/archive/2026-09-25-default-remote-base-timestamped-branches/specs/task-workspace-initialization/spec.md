## MODIFIED Requirements

### Requirement: Initialize a task workspace from explicit local repositories
The CLI SHALL provide `taskflow init <task-id>` with an explicit tasks root and one or more unique `--repo <name>=<path>` values. It MUST create a task control directory containing `taskflow.yaml` and `.taskflow/state.json` without creating branches, worktrees, commits, or fetching remotes. For each repository, default values MUST use its resolved `origin/<default-branch>` as `base` and the shared `feature/<lowercase-task-id>` branch. An explicit remote-default reference or branch value already present in configuration remains authoritative.

#### Scenario: Initialize three valid repositories
- **WHEN** a user initializes a new task with three existing local Git repositories whose `origin/HEAD` references are usable
- **THEN** the CLI writes task metadata with each repository's remote default base and one shared task branch, without modifying Git state in any source repository

#### Scenario: Reject a non-Git repository
- **WHEN** an explicit repository path is not an existing Git work tree
- **THEN** the CLI returns a configuration error and does not create a partial task workspace

#### Scenario: Reject a repository without remote default
- **WHEN** an explicit repository path is a Git work tree but `origin/HEAD` is missing or unresolved
- **THEN** the CLI returns a diagnostic before writing `taskflow.yaml` or `.taskflow/state.json`

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
