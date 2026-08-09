## Context

`start --execute` writes `.taskflow/state.json` after each action. It must use persisted action outcomes alongside Git inspection so retries do not repeat compatible completed fetches or worktree actions.

The change must preserve the task lock, source-branch locks, read-only preflight, atomic state writes, stable exit codes, and the existing no-rollback safety model.

## Goals / Non-Goals

**Goals:**

- Resume from compatible persisted state without redoing completed safe actions.
- Reconcile persisted completion with current filesystem and Git facts before mutation.
- Detect configuration/state incompatibility before changing the workspace.
- Keep retries idempotent and observable through existing reports and state files.

**Non-Goals:**

- Cross-repository rollback.
- Automatic cleanup or repair of mismatched worktrees.
- Parallel execution or Git query performance changes.
- Changing public exit-code meanings or restoring OpenSpec runtime integration.

## Decisions

- Add a configuration digest to persisted state. A retry with a different normalized task configuration returns a structured state conflict before mutation. An empty digest is treated as legacy state and backfilled once.
- Load and validate state after locks and read-only preflight, but before the first mutation. Malformed or incompatible state is rejected without rewriting it.
- Treat actual facts as authoritative for action completion: an existing managed worktrees directory completes the directory action; a matching configured worktree completes the worktree action; a completed fetch is reusable only while the configured base ref is still available.
- Preserve prior repository outcomes and update only actions being resumed. Persist the compatible `starting` state before mutation and atomically persist after each completion or failure.
- Use the existing preflight to reject target/branch ownership conflicts. A missing previously completed worktree is safe to recreate; a mismatched existing target remains a conflict and is never overwritten.

## Risks / Trade-offs

- [State from older versions has no digest] → accept an empty digest as legacy only, then persist the current digest before mutation.
- [A remote ref can change after a prior fetch] → reuse a completed fetch only if the configured base ref is currently resolvable; otherwise fetch again.
- [State may claim completion while the workspace was manually changed] → reconcile actual facts and let preflight reject unsafe worktree/branch mismatches.
- [State file can be corrupted] → return a state incompatibility diagnostic and preserve the file for manual recovery.

## Migration Plan

No explicit migration is required. Existing schema-version-1 state without `configDigest` remains readable; the next compatible execute start backfills the digest. Incompatible or malformed state is reported without mutation.
