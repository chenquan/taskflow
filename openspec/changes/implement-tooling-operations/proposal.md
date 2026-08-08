## Why

After `start`, users need to enter a managed worktree with either Codex or Claude and observe all repositories from one task report. Without session ownership and aggregated validation, two tools can write concurrently or users must inspect each repository manually.

## What Changes

- Add tool adapters and Cobra `open <task-id> --tool codex|claude` with safe launch arguments.
- Add task session leases that prevent concurrent write sessions and release on process exit.
- Add `status`, `validate`, and `finish --dry-run` aggregate reports.
- Execute configured checks with bounded dependency-aware ordering and preserve machine-readable results.

## Capabilities

### New Capabilities

- `development-tool-sessions`: Launch Codex/Claude with managed directories and mutually exclusive leases.
- `aggregate-status-validation`: Aggregate Git, OpenSpec, task progress, configured checks, and finish readiness.

### Modified Capabilities

None.

## Impact

Adds development-tool adapters, lease persistence, status/validation application services, command bindings, and tests. No archive, cleanup, push, or PR execution is added.
