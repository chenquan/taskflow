## MODIFIED Requirements

### Requirement: Preflight destructive cleanup
Execute-mode delete MUST acquire the task lock and all configured source-branch locks, verify every owned target's live Git identity, reject dirty worktrees—including worktrees dirty only because of copied local overlay files—unless --force is supplied, require a resolvable source default branch and protect it, and reject unmanaged task-directory entries before Git mutation.

#### Scenario: Reject unsafe cleanup before mutation
- **WHEN** a target is dirty, mismatched, registered at another path, uses a protected branch, or the task directory contains unmanaged content
- **THEN** delete returns a deterministic conflict and preserves the task, worktrees, branches, and files

#### Scenario: Require force for copied overlay files
- **WHEN** an owned worktree contains copied overlay files and delete is run without `--force --execute`
- **THEN** delete reports the dirty-worktree conflict and does not remove the worktree or overlay files

#### Scenario: Allow explicitly forced cleanup
- **WHEN** an owned worktree containing copied overlay files passes all identity checks and the user supplies `--force --execute`
- **THEN** delete may remove the owned worktree, branch, overlay metadata, and task directory according to the existing destructive cleanup contract
