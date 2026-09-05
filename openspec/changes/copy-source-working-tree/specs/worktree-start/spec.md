## MODIFIED Requirements

### Requirement: Plan a complete start operation
The CLI SHALL provide `taskflow create <task-id> --dry-run` and list every configured repository's resolved target, Worktree action, and source-copy action in stable declaration order. Dry-run MUST not modify files, lock directories, taskflow.yaml, worktrees, branches, or Git state.

#### Scenario: Dry-run three repositories
- **WHEN** a valid task has three repositories and the user requests create dry-run
- **THEN** the result lists each repository's Worktree and source-copy plan deterministically and leaves the task directory, source checkout, and Git state unchanged

### Requirement: Create safe managed worktrees
Execute mode MUST acquire the task lock, acquire all required source-branch locks, and complete a read-only preflight for every repository before writing taskflow.yaml or invoking a mutating command. Preflight MUST verify source identity, locally resolvable base, branch occupancy, target containment, and target identity. Execute mode MUST create each missing Worktree with the configured branch and base using an argument-array Git invocation, register it without a base checkout, populate its index from HEAD, copy the complete source working directory excluding all `.git` entries, and never delete or overwrite a mismatched pre-existing target.

#### Scenario: Create and copy a missing Worktree
- **WHEN** every repository passes preflight, a target path is absent, and its branch is available
- **THEN** create writes the desired configuration, registers the Worktree from the configured base, and copies the source working directory into the target while preserving the target Git metadata

#### Scenario: Reject a mismatched target
- **WHEN** the target path exists but is not the expected Worktree
- **THEN** create returns conflict code 5 and preserves taskflow.yaml and the existing directory byte-for-byte

### Requirement: Start is idempotent
Create MUST derive Worktree completion from live Git facts and the source-copy status. An existing Worktree is complete only when its canonical source/common Git directory, configured branch, configured target path, and source-copy status match. A matching Worktree with a complete copy MUST be reused without another `git worktree add` invocation or source copy.

#### Scenario: Repeat create
- **WHEN** create is run again after successful Worktree creation and source copying
- **THEN** it reports every Worktree and source-copy action as reused and creates no duplicate branch, directory, or copy
