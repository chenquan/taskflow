## Context

The command service distinguishes a change directory from completed OpenSpec work, while finish readiness depends on validation. Configuration currently interprets any successful `git rev-parse --is-inside-work-tree` process as a worktree. Initialization also needs the final task path to normalize worktree paths, but creates that path before validating the constructed task.

## Goals / Non-Goals

**Goals:**

- Treat missing, unreadable, or unchecked OpenSpec task lists as validation blockers.
- Accept only source directories for which Git reports `true` for `--is-inside-work-tree`.
- Reject invalid `init` inputs before creating the final task directory.

**Non-Goals:**

- Evaluate arbitrary OpenSpec artifact schemas beyond the task checklist.
- Add rollback for failures after a valid initialization has begun writing metadata.
- Change task configuration fields or shell-execution policy.

## Decisions

- Add a focused OpenSpec checklist inspection helper rather than invoking an OpenSpec CLI status command. The managed `tasks.md` path is deterministic, preserves offline validation, and distinguishes unreadable files from complete task lists.
- Capture `git rev-parse --is-inside-work-tree` output and require the trimmed value `true`. Process success alone is insufficient because bare repositories return `false` successfully.
- Validate the generated initialization task against the existing, canonical tasks root before assigning its final child path. The normalized relative worktree paths are unchanged, so this reuses the full configuration validator without creating `<tasks-root>/<task-id>` on rejected input.

## Risks / Trade-offs

- [A nonstandard task list has no unchecked Markdown checkbox] → It is considered complete; the check intentionally enforces the documented checklist convention only.
- [Git emits unexpected output] → Validation rejects it with the existing not-a-worktree diagnostic rather than attempting later mutations.
- [A write fails after validation] → Existing atomic writes and initialization locking still preserve no-overwrite behavior; cleanup of valid-but-failed initialization is out of scope.
