## ADDED Requirements

### Requirement: Create an independent OpenSpec change per worktree
Execute mode SHALL invoke OpenSpec in each managed worktree using the configured lower-kebab-case change ID. It MUST treat a pre-existing matching change as complete and MUST stop when the existing change identity is incompatible.

#### Scenario: Create changes in dependency order
- **WHEN** all required worktrees exist and OpenSpec is available
- **THEN** start creates one independent change directory in each worktree in repository dependency order

#### Scenario: Reuse an existing change
- **WHEN** the configured change directory already exists in the expected worktree
- **THEN** start reports it as reused and does not create a second change
