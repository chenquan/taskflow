## MODIFIED Requirements

### Requirement: Initialize a task workspace from explicit local repositories
The CLI SHALL provide `specflow init <task-id>` with an explicit tasks root, one or more unique `--repo <name>=<path>` values, and a primary repository selection. It MUST create a requirement control directory containing `requirement.md`, `specflow.yaml`, `.specflow/inventory.json`, and `.specflow/state.json` without creating branches, worktrees, changes, commits, or fetching remotes.

#### Scenario: Initialize three valid repositories
- **WHEN** a user initializes a new task with three existing local Git repositories and a valid primary repository
- **THEN** the CLI writes task metadata without OpenSpec runtime fields and reports success without modifying Git state in any source repository

#### Scenario: Reject a non-Git repository
- **WHEN** an explicit repository path is not an existing Git work tree
- **THEN** the CLI returns a configuration error and does not create a partial task workspace
