## Why

`start --execute` persists action outcomes, but a retry rebuilds a fresh pending state and relies mainly on Git idempotency. This makes recovery less explicit and can repeat completed fetches or probes after a partial failure.

The command should use persisted progress together with current Git/worktree facts so retries are predictable, safe, and observable.

## What Changes

- Resume `start --execute` from persisted action outcomes when the task state is compatible.
- Reconcile persisted outcomes with actual directory and worktree state before deciding whether to skip or execute an action.
- Preserve completed actions and update only the action currently being resumed.
- Report structured state-conflict diagnostics when persisted state and the actual managed workspace disagree.
- Add regression coverage for partial failure, retry, completed-action skipping, and incompatible state.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `resumable-action-execution`: retries must derive completed actions from persisted state and current managed Git/filesystem facts, while refusing unsafe state conflicts.

## Impact

- Updates `internal/app` start execution and state reconciliation.
- Extends domain action/state reporting without changing existing exit-code meanings.
- Adds E2E and unit tests; no external dependencies, migrations, or OpenSpec runtime integration.
