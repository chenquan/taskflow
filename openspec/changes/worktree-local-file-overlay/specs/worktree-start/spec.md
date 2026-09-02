## MODIFIED Requirements

### Requirement: Plan a complete start operation
The CLI SHALL provide `taskflow create <task-id> --dry-run` and list every configured repository's resolved target, worktree `create` or `reuse` action, and local overlay action in stable declaration order. Dry-run MUST not modify files, lock directories, taskflow.yaml, ownership metadata, worktrees, branches, or Git state.

#### Scenario: Dry-run three repositories and overlays
- **WHEN** a valid task has three repositories and one or more local overlay paths and the user requests create dry-run
- **THEN** the result lists all worktree and overlay actions deterministically and leaves the task directory, source files, and Git state unchanged

### Requirement: Create safe managed worktrees
Execute mode MUST acquire the task lock, acquire all required source-branch locks, and complete a read-only preflight for every repository and selected overlay before writing taskflow.yaml or invoking a mutating command. Preflight MUST verify source identity, locally resolvable base, branch occupancy, target containment and identity, overlay source paths and file types, overlay snapshot hashes, and base-tree collisions. Execute mode MUST create each missing worktree using argument-array Git invocation, materialize only its declared overlay, and never delete or overwrite a mismatched target or existing file.

#### Scenario: Create a missing worktree and overlay
- **WHEN** every repository and overlay passes preflight, a target path is absent, and its branch is available
- **THEN** create writes the desired configuration and ownership snapshot, creates the worktree at the contained target path from the base, and publishes the selected overlay files

#### Scenario: Reject an overlay collision before mutation
- **WHEN** a selected overlay path conflicts with a tracked base path or an unsafe target path
- **THEN** create returns an overlay diagnostic before writing taskflow.yaml or changing Git or filesystem state

### Requirement: Start is idempotent
Create MUST derive worktree completion from live Git facts and overlay completion from the optional ownership snapshot. An existing worktree is complete only when its canonical source/common Git directory, configured branch, and configured target path match; an overlay is complete only when its recorded snapshot is complete and all files still match their recorded materialization. A matching worktree with a complete overlay MUST be reused without another `git worktree add` invocation or overlay refresh. A pending overlay MAY be repaired without recreating the worktree.

#### Scenario: Repeat create
- **WHEN** create is run again after successful worktree and overlay creation
- **THEN** it reports every worktree and overlay as reused or complete and creates no duplicate branch, directory, or copied file

#### Scenario: Repair an incomplete overlay
- **WHEN** a matching Taskflow-owned worktree has a pending overlay and one selected file is missing
- **THEN** create reports a repair action and copies only the missing file
