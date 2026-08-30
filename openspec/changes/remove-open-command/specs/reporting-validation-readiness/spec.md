## REMOVED Requirements

### Requirement: Report create, open, and delete operational results
**Reason**: The `open` command is removed; launch results no longer exist as CLI output, so the reporting requirement covers create and delete only.
**Migration**: Native tool launches happen outside Taskflow; their output is the tool's own terminal output.

## ADDED Requirements

### Requirement: Report create and delete operational results
Create and delete SHALL expose their current action, conflict, and cleanup results through the common text and JSON output contract without validation or readiness history.

#### Scenario: Report a create action
- **WHEN** create previews or executes a repository reconciliation
- **THEN** output includes the repository and its create, reuse, or failure result

#### Scenario: Report a delete action
- **WHEN** delete previews or executes an ownership-checked cleanup
- **THEN** output includes each worktree, local branch, and task-directory action and its result
