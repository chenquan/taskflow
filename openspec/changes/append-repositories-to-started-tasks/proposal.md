## Why

Once a task is initialized or started, Taskflow offers no safe way to attach another repository. Today the only path is to edit `taskflow.yaml` by hand, which leaves `state.json` carrying a stale configuration digest and trips `STATE_CONFLICT` on the next `start`, and silently lets an outdated validation report masquerade as the current result. Teams that discover a missing repository mid-task are forced into destructive workarounds. We need an explicit, append-only command that grows the task configuration without invalidating the work already done.

## What Changes

- Add `taskflow repo add <task-id> --repo <name>=<absolute-path> [--depends-on <repo>] [--dry-run]`, a metadata-only append that validates the repository path, Git status, name uniqueness, and dependency references, then appends to the task configuration without creating any worktree.
- New appended repositories reuse the same defaults as `init`: `base: HEAD`, `branch: feature/<task-id>`, `worktree: worktrees/<name>`, and no checks. Dependencies default to empty and may reference only existing repositories.
- The append is append-only: it never modifies or removes existing repositories and never changes the primary repository.
- `repo add` is allowed only while the task is in the `initialized`, `started`, or `failed` phase, and acquires the task lock to prevent concurrent appends or concurrent `start`.
- The append performs a controlled migration that updates the configuration digest across `taskflow.yaml`, `inventory.json`, and `state.json` in one atomic step: existing repository action outcomes and worktrees are preserved, the new repository receives a pending action state, and the persisted digest advances to the new configuration so the next `start --execute` reconciles only the new repository.
- `status` no longer treats a persisted validation report as current when its configuration digest no longer matches the task configuration. It reports `validationStale: true` and omits the stale per-repository results, while leaving `validation.json` on disk as historical evidence. The next `validate` regenerates a report for the current configuration.
- `start --execute` still validates the full configuration digest, but the authorized `repo add` migration is the only path that may advance it; worktree creation continues to require the dry-run and explicit execute flow.

## Capabilities

### New Capabilities
- `repository-append`: Append-only metadata mutation that grows an existing task with another repository, gates it by task phase, persists the change atomically across configuration, inventory, and state with an advanced configuration digest, and supports a dry-run preview of the resolved repository and the subsequent start actions.

### Modified Capabilities
- `resumable-action-execution`: Start still rejects a persisted configuration digest that differs from the normalized task configuration; the authorized `repo add` migration is added as the only way to advance that digest while preserving existing action outcomes.
- `aggregate-status-validation`: Status must report a stale validation report when its configuration digest no longer matches the task configuration and must not present the stale report as a current per-repository result.

## Impact

- Adds the `repo add` application service and `repo` command group in `cmd`, plus a snapshot-and-restore writer for `taskflow.yaml`, `inventory.json`, and `state.json`.
- Extends the status domain model with a `validationStale` flag and updates status to compare the persisted validation report digest against the current configuration digest.
- Updates README and the Taskflow skill with the append workflow, phase constraints, and the dry-run then execute sequence.
- No external dependencies, no exit-code meaning changes, and no migration of existing task directories beyond the digest advancement performed by the command itself.
