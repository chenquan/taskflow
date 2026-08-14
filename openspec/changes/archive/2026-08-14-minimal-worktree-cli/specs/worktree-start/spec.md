## MODIFIED Requirements

### Requirement: Plan a complete start operation
The CLI SHALL provide `taskflow create <task-id> --dry-run` and list every configured repository's resolved target and `create` or `reuse` action in stable declaration order. Dry-run MUST not modify files, lock directories, taskflow.yaml, worktrees, branches, or Git state.

#### Scenario: Dry-run three repositories
- **WHEN** a valid task has three repositories and the user requests create dry-run
- **THEN** the result lists all three planned actions deterministically and leaves the task directory and Git state unchanged

### Requirement: Create safe managed worktrees
Execute mode MUST acquire the task lock, acquire all required source-branch locks, and complete a read-only preflight for every repository before writing taskflow.yaml or invoking a mutating command. Preflight MUST verify source identity, locally resolvable base, branch occupancy, target containment, and target identity. Execute mode MUST create each missing worktree using argument-array Git invocation with the configured branch and base, and never delete or overwrite a mismatched target.

#### Scenario: Create a missing worktree
- **WHEN** every repository passes preflight, a target path is absent, and its branch is available
- **THEN** create writes the desired configuration and creates the worktree at the contained target path from the configured base

#### Scenario: Reject a mismatched target
- **WHEN** the target path exists but is not the expected worktree
- **THEN** create returns conflict code 5 and preserves taskflow.yaml and the existing directory byte-for-byte

### Requirement: Start is idempotent
Create MUST derive completion from live Git facts. An existing worktree is complete only when its canonical source/common Git directory, configured branch, and configured target path match. A matching worktree MUST be reused without another `git worktree add` invocation.

#### Scenario: Repeat create
- **WHEN** create is run again after successful worktree creation
- **THEN** it reports every worktree as reused and creates no duplicate branch or directory
