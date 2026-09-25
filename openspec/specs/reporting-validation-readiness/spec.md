## Purpose

Define current operational results for create and delete without validation history or readiness conclusions.
## Requirements
### Requirement: Report create and delete operational results
Create and delete SHALL expose their current action, conflict, and cleanup results through the common text and JSON output contract without validation or readiness history.

#### Scenario: Report a create action
- **WHEN** create previews or executes a repository reconciliation
- **THEN** output includes the repository and its create, reuse, or failure result

#### Scenario: Report a delete action
- **WHEN** delete previews or executes an ownership-checked cleanup
- **THEN** output includes each worktree, local branch, and task-directory action and its result
