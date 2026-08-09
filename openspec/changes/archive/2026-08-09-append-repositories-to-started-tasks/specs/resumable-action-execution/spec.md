## ADDED Requirements

### Requirement: Accept authorized configuration advancement
Start MUST continue to reject a persisted configuration digest that differs from the normalized task configuration. The `repo add` migration is the only authorized way to advance that digest: after it advances the digest while preserving existing action outcomes, a subsequent `start --execute` MUST accept the matching digest and reconcile only repositories whose actions are still pending, reusing existing worktrees and creating worktrees only for appended repositories.

#### Scenario: Resume after an authorized append
- **WHEN** a started task appends a second repository via `repo add` and the user runs `start --execute`
- **THEN** start accepts the advanced digest, reuses the first repository's existing worktree, and creates only the appended repository's worktree

#### Scenario: Repeated start is idempotent after append
- **WHEN** `start --execute` runs again after the appended repository's worktree exists
- **THEN** start reports both worktrees as already complete and creates no duplicate branch or directory
