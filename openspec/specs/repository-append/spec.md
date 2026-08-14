## Purpose

Define append-only repository growth through the create reconciliation command.
## Requirements
### Requirement: Fold repository growth into create
Create SHALL accept new unique repository declarations for an existing task, preserve existing declaration order, and reconcile the newly declared worktree without an independent repository command.

#### Scenario: Append through create
- **WHEN** an existing task receives a new repository with `create --repo`
- **THEN** create previews or creates only the new worktree and preserves existing repositories
