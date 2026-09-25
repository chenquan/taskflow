## Purpose

Define complete create dry-run planning and safe, idempotent managed worktree reconciliation.
## Requirements
### Requirement: Plan a complete start operation
The CLI SHALL provide `taskflow create <task-id> --dry-run` and list every configured repository's resolved target, Worktree action (`create` or `reuse`), and source-copy action (`copy`, `repair`, or `reuse`) with the `.worktreeinclude` pattern count in stable declaration order. Dry-run MUST not modify files, lock directories, taskflow.yaml, worktrees, branches, or Git state, and MUST NOT enumerate or read source contents beyond the manifest file itself.

#### Scenario: Dry-run three repositories
- **WHEN** a valid task has three repositories and the user requests create dry-run
- **THEN** the result lists each repository's Worktree and source-copy plan deterministically and leaves the task directory, source checkout, and Git state unchanged

#### Scenario: Dry-run validates manifests without walking
- **WHEN** a source repository has a missing or syntactically invalid `.worktreeinclude` and the user requests create dry-run
- **THEN** the command returns the manifest diagnostic for that repository without creating or modifying any state

### Requirement: Create safe managed worktrees
Execute mode MUST acquire the task lock, acquire all required source-branch locks, and complete a read-only preflight for every repository before writing taskflow.yaml or invoking a mutating command. Preflight MUST verify source identity, locally resolvable base, branch occupancy, target containment, target identity, and `.worktreeinclude` presence and syntax. Execute mode MUST create each missing Worktree with the configured branch and base using an argument-array Git invocation with a normal base checkout, then copy exactly the manifest-matched source paths into the target while preserving the target Git metadata, and never delete or overwrite a mismatched pre-existing target.

#### Scenario: Create and overlay a missing Worktree
- **WHEN** every repository passes preflight, a target path is absent, and its branch is available
- **THEN** create writes the desired configuration, creates the Worktree with a base checkout, and copies the `.worktreeinclude`-matched source paths into the target

#### Scenario: Reject a mismatched target
- **WHEN** the target path exists but is not the expected Worktree
- **THEN** create returns conflict code 5 and preserves taskflow.yaml and the existing directory byte-for-byte

#### Scenario: Missing manifest blocks creation
- **WHEN** a source repository has no `.worktreeinclude` and execute would create its Worktree
- **THEN** create returns `SOURCE_COPY_MANIFEST_MISSING` for that repository before Git mutation and preserves all existing state

### Requirement: Start is idempotent
Create MUST derive Worktree completion from live Git facts and the source-copy status. An existing Worktree is complete only when its canonical source/common Git directory, configured branch, configured target path, and source-copy status match. A matching Worktree with a complete copy MUST be reused without another `git worktree add` invocation or source copy. A matching Taskflow-owned Worktree with a pending source copy MUST be repaired by rerunning the manifest-driven overlay.

#### Scenario: Repeat create
- **WHEN** create is run again after successful Worktree creation and source copying
- **THEN** it reports every Worktree and source-copy action as reused and creates no duplicate branch, directory, or copy

#### Scenario: Pending copy is repaired
- **WHEN** a Taskflow-owned Worktree matches but its recorded source copy is pending
- **THEN** create reruns the overlay of manifest-matched paths into that target and marks the copy complete
