## Context

Current `Result.Data` is JSON-only, validation uses input order, status omits its session lease, and finish does not block dirty managed worktrees. Configuration accepts malformed durations and change IDs, allowing failures after mutation starts.

## Goals / Non-Goals

**Goals:** expose all computed operational data in text mode, validate deterministically in dependency order, report active sessions, and fail invalid readiness/configuration early.

**Non-Goals:** change YAML shape, remove stale leases during status, or add shell execution.

## Decisions

- Text rendering prints a `data:` section containing indented JSON for any non-nil data, preserving a single representation for all commands.
- Validation obtains repositories through `plan.Order`; inability to order is configuration failure.
- Session exposes a read-only `Active(root)` that returns a live lease or nil without deleting files.
- Finish reads status entries and appends a `DIRTY_WORKTREE` error for each dirty repository; any error maps to validation exit code.
- Non-empty timeout values use `time.ParseDuration`; change IDs use `^[a-z0-9]+(?:-[a-z0-9]+)*$`.

## Risks / Trade-offs

- [Text JSON is verbose] → it faithfully renders all fields and remains stable as result data grows.
- [PID liveness races] → status remains a point-in-time snapshot and does not mutate the lease.

## Migration Plan

No migration. Existing malformed configuration becomes an early validation error.
