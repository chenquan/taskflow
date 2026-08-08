## Context

Phase 1 stores canonical sources and desired worktree/change identifiers. Phase 2 must turn that intent into isolated working directories while preserving explicit ownership boundaries and safe recovery. Git and OpenSpec are external CLIs with evolving output, so adapters must expose stable internal models and never compose shell strings.

## Goals / Non-Goals

**Goals:**

- Produce a full, human-readable execution plan before any mutation.
- Create worktrees in dependency order and changes inside the corresponding worktrees.
- Detect and reuse exactly matching existing worktrees/changes.
- Persist progress atomically after each action and stop safely on failure.

**Non-Goals:**

- Automatic rollback or deletion of worktrees, branches, or OpenSpec artifacts.
- Fetching by default, commits, pushes, or parallel execution in this phase.

## Decisions

- Represent each operation as an action with stable ID, description, check, execute, and completed state. This makes dry-run and recovery use the same plan.
- Parse `git worktree list --porcelain` into canonical path/branch/source facts. A target that exists but does not match the configured source and branch is a hard conflict.
- Use `git -C <source> worktree add -b <branch> <target> <base>` only after validation. OpenSpec creation runs with `openspec new change <id> --json` in the worktree and is considered complete only when the change directory exists.
- Topologically sort repositories using the existing dependency validator; use lexical order within a layer for deterministic plans. Avoid parallel writes until a later phase adds bounded concurrency.
- Keep `state.json` a cache/audit trail. On every start, inspect the actual filesystem, Git worktree list, and change directory before deciding an action is complete.

## Risks / Trade-offs

- [Git command output varies] → parse only porcelain markers and return compatibility errors for missing required markers.
- [A partially-created worktree remains after an error] → record the completed action and never delete it automatically; the next run re-inspects it.
- [OpenSpec CLI is unavailable] → dry-run can still render the complete plan, while execute fails before creating a change and reports the exact tool error.
- [A branch is already used elsewhere] → inspect all worktrees and reject the action before invoking `git worktree add`.

## Migration Plan

Existing initialized tasks need no migration. The first `start` writes phase and repository action state into `.specflow/state.json`; rerunning is the recovery path. No existing worktree is removed.

## Open Questions

- Fetch policy remains the existing `execution.fetch` configuration and is not enabled implicitly by this change.
