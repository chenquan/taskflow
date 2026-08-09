## Purpose

Define complete dry-run planning and safe, idempotent managed worktree creation.

## Requirements

### Requirement: Plan a complete start operation
The CLI SHALL provide `taskflow start <task-id> --dry-run` and list directory, fetch (when configured), and worktree actions in dependency order before execution. Dry-run MUST not modify files, task state, or Git state.

#### Scenario: Dry-run three repositories
- **WHEN** a valid task has three repositories and the user requests dry-run
- **THEN** the result lists all planned directory, fetch, and worktree actions in deterministic dependency order and leaves the sources and task state unchanged

### Requirement: Create safe managed worktrees
Execute mode MUST acquire the task lock, acquire all required source-branch locks, and complete a read-only preflight for every repository before writing state or invoking a mutating command. Preflight MUST verify source identity, base and remote availability, branch occupancy, and target identity. Execute mode MUST create each configured worktree using argument-array Git invocation with the configured branch and base, reject an existing target or branch that does not match the configured repository, and never delete or overwrite it.

#### Scenario: Create a missing worktree
- **WHEN** every repository passes preflight, a target path is absent, and its branch is available
- **THEN** start creates the worktree at the contained target path from the configured base

#### Scenario: Reject a mismatched target
- **WHEN** the target path exists but is not the expected worktree
- **THEN** start returns conflict code 5 and preserves task state and the existing directory byte-for-byte

### Requirement: Start is idempotent
An existing worktree is complete only when its canonical source/common Git directory and configured branch match. A matching worktree MUST be reused without another `git worktree add` invocation.

#### Scenario: Repeat start
- **WHEN** start is run again after successful worktree creation
- **THEN** it reports the worktree action as already complete and makes no duplicate branch or directory
