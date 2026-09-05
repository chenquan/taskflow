## ADDED Requirements

### Requirement: Copy the complete source working directory
When create provisions a missing Worktree, Taskflow SHALL copy every source working-directory entry beneath the configured source root into the new target, including tracked files with uncommitted changes, untracked files, and ignored files. All source `.git` entries — the root entry and any `.git` entry at a nested depth — SHALL be excluded, and the target Worktree's own Git metadata SHALL remain intact. Before copying, Taskflow SHALL populate the new Worktree's index from HEAD so that copied tracked modifications appear as normal unstaged working-tree changes.

#### Scenario: Copy a complete source snapshot
- **WHEN** a valid task creates a missing Worktree from a source containing committed files, uncommitted tracked changes, untracked files, and ignored files
- **THEN** the target working-tree contents match the source contents for all entries except source `.git` metadata

#### Scenario: Preserve the target Worktree metadata
- **WHEN** the source and target both contain a `.git` entry after Worktree registration
- **THEN** the source `.git` entry is not copied over the target `.git` entry and the target remains attached to its configured branch and Git common directory

#### Scenario: Exclude nested Git metadata
- **WHEN** the source working directory contains a nested `.git` entry belonging to another checkout
- **THEN** that entry is not copied into the target and the nested working files themselves are copied

#### Scenario: Show copied tracked changes as unstaged changes
- **WHEN** the source contains uncommitted tracked modifications and the copy completes
- **THEN** the target reports those files as unstaged modifications with no staged deletions, and reports an empty status when the source matches the configured base

### Requirement: Copy only a newly created or pending Taskflow Worktree
Taskflow SHALL run the complete source copy only after it creates a missing target or when it repairs a Taskflow-owned target whose source-copy status is pending. A matching existing Worktree with a complete source-copy status SHALL be reused without copying, and a matching Worktree without Taskflow ownership SHALL never receive an implicit source copy.

#### Scenario: Reuse a completed source copy
- **WHEN** create runs again after a Taskflow-created target has a complete source-copy status
- **THEN** create reports reuse and does not refresh the target from later source changes

#### Scenario: Do not populate a matching manual Worktree
- **WHEN** a manually created Worktree matches the configured source, branch, and target
- **THEN** create reports reuse and leaves the target contents unchanged

### Requirement: Retry a pending complete source copy
Taskflow SHALL persist a repository-level pending or complete source-copy status for each Taskflow-created target. A retry of a pending target SHALL rerun the complete source copy; when the pending target is absent, Taskflow SHALL register the missing Worktree before copying. Taskflow SHALL mark the status complete only after success and SHALL report a partial failure while the status remains pending.

#### Scenario: Retry an interrupted copy
- **WHEN** Worktree registration succeeds but the source copy fails before all entries are copied, and create is run again after the external fault is fixed
- **THEN** create reuses the matching target, reruns the source copy, and marks the source-copy status complete only after the retry succeeds

#### Scenario: Register a missing pending target before retrying the copy
- **WHEN** a pending source-copy status exists but the target directory is absent because registration itself failed
- **THEN** the retry registers the missing Worktree from the configured base and then reruns the complete source copy

#### Scenario: Preserve completed-copy immutability
- **WHEN** a source file changes after the source-copy status is complete
- **THEN** a later create does not replace the corresponding target file
