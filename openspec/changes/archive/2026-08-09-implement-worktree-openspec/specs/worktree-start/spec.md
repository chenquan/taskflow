## ADDED Requirements

### Requirement: Plan a complete start operation
The CLI SHALL provide `specflow start <task-id> --dry-run` and list directory, fetch (when configured), worktree, and OpenSpec change actions in dependency order before execution. Dry-run MUST not modify files, Git state, or OpenSpec state.

#### Scenario: Dry-run three repositories
- **WHEN** a valid task has three repositories and the user requests dry-run
- **THEN** the result lists all planned worktree and change actions in deterministic dependency order and leaves the sources and task state unchanged

### Requirement: Create safe managed worktrees
Execute mode MUST create each configured worktree using argument-array Git invocation, with the configured branch and base. It MUST reject an existing target or branch that does not match the configured repository and MUST never delete or overwrite it.

#### Scenario: Create a missing worktree
- **WHEN** a target path is absent and its branch is available
- **THEN** start creates the worktree at the contained target path from the configured base

#### Scenario: Reject a mismatched target
- **WHEN** the target path exists but is not the expected worktree
- **THEN** start stops with a conflict and preserves the existing directory

### Requirement: Start is idempotent
An existing worktree is complete only when its canonical source/common Git directory and configured branch match. A matching worktree MUST be reused without another `git worktree add` invocation.

#### Scenario: Repeat start
- **WHEN** start is run again after successful worktree creation
- **THEN** it reports the worktree action as already complete and makes no duplicate branch or directory
