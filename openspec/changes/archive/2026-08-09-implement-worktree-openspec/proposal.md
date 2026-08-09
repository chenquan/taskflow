## Why

The initialized task workspace currently records repositories but cannot create the isolated worktrees and per-repository OpenSpec changes that the plan requires for safe multi-repository development. Phase 2 closes that gap with a deterministic, resumable start operation.

## What Changes

- Add `specflow start <task-id> --dry-run|--execute` with a complete action plan.
- Add read-only worktree inspection and safe `git worktree add -b` execution.
- Add an OpenSpec adapter that detects capabilities and creates one change in each managed worktree.
- Execute repositories in dependency-topological order and persist action outcomes for recovery.
- Reuse already-correct worktrees and changes; stop on mismatches without deleting user data.

## Capabilities

### New Capabilities

- `worktree-start`: Plan and execute dependency-ordered, idempotent Git worktree creation.
- `openspec-change-management`: Detect OpenSpec support and create/check a change in each managed worktree.
- `resumable-action-execution`: Persist per-action state and resume after a failed action without claiming rollback.

### Modified Capabilities

None. The prior initialization behavior remains compatible; this change adds the next lifecycle command.

## Impact

Adds planner/executor application services, Git worktree adapter methods, OpenSpec adapter methods, state transitions, and Cobra `start` command bindings. `--dry-run` remains non-mutating; execute mode mutates only configured task worktrees and their OpenSpec planning directories. Commit, push, archive, cleanup, and PR operations remain out of scope.
