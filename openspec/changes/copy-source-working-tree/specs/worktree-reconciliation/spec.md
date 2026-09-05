## MODIFIED Requirements

### Requirement: Reconcile worktrees idempotently and safely
Create SHALL classify each repository as `create` when its configured target is absent, `copy` when a new target must receive the source working-tree snapshot, `repair` when a matching Taskflow-owned target has a pending source-copy status, or `reuse` when the Worktree and source copy are complete. It MUST reject an existing mismatched target or occupied branch before mutation and MUST never delete, move, reset, or overwrite an existing user path.

#### Scenario: Reuse a matching completed Worktree
- **WHEN** create is run after a matching Worktree and complete source copy already exist
- **THEN** create succeeds with Worktree and source-copy reuse results and does not invoke `git worktree add` or copy files

#### Scenario: Repair a pending source copy
- **WHEN** a matching Taskflow-owned Worktree has a pending source-copy status
- **THEN** create reports repair, does not invoke `git worktree add` again, and reruns the complete source copy

#### Scenario: Reuse a matching manual Worktree
- **WHEN** a user-created Worktree structurally matches the configured source, target, and branch but has no Taskflow ownership entry
- **THEN** create reports Worktree reuse and does not copy source files into it

#### Scenario: Reject a mismatched target
- **WHEN** the configured target exists but is not the expected Worktree from the expected source and branch
- **THEN** create returns a deterministic Worktree conflict before writing taskflow.yaml or changing Git state

### Requirement: Execute only after complete preflight
Execute-mode create MUST acquire the task lock and all required source/branch locks, inspect every source, base ref, branch occupancy, target path, target identity, and source-copy target boundary before writing an initial taskflow.yaml or invoking a mutating Git command. A source or branch lock MUST be acquired in deterministic order and released when create returns.

#### Scenario: Preflight blocks before mutation
- **WHEN** any repository has an unavailable base, invalid source, branch conflict, mismatched target, or unsafe source-copy boundary
- **THEN** create returns the specific diagnostic and leaves taskflow.yaml, all Git Worktrees, and all target files unchanged
