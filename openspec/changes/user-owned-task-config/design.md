## Context

Taskflow currently uses `taskflow.yaml` as the desired configuration, but `create --repo` also mutates an existing configuration by appending repositories. This makes the create command both a worktree reconciler and an imperative configuration editor. The tool is primarily called by AI agents, so the interface must remain explicit, machine-readable, previewable, and easy to retry while leaving the durable topology visible to users.

## Goals / Non-Goals

**Goals:**

- Keep `create --repo` as a convenient first-task bootstrap API for AI agents.
- Make an existing `taskflow.yaml` the sole user/agent-owned topology input.
- Reconcile direct configuration edits without rewriting the file or deleting unlisted worktrees.
- Preserve all existing Git safety, locking, dry-run, JSON output, and retry guarantees.

**Non-Goals:**

- Add a new configuration-management command.
- Migrate old append invocations or old task directories.
- Change branch/base defaults, worktree identity, or CLI launch behavior.

## Decisions

### Separate bootstrap from reconciliation

Configuration existence is the boundary. When `taskflow.yaml` does not exist, `create --repo` constructs and validates the initial desired configuration. When it exists, `create` loads it and requires no repository arguments. Supplying `--repo` for an existing task returns `CONFIG_EDIT_REQUIRED` before source inspection, lock acquisition, configuration writes, or Git mutation.

This is preferred over continuing append semantics because it removes implicit merge behavior and gives an AI agent one durable configuration artifact to inspect and edit. It is preferred over removing `--repo` entirely because first-time AI tool calls remain concise and typed.

### Make existing configuration read-only to create

Execute-mode create writes `taskflow.yaml` only for a new task after complete preflight. For an existing task, it never serializes normalized defaults or rewrites the file. Configuration changes are made outside the reconciler, then validated and materialized by a normal create invocation.

The reconciler iterates only the repositories declared in the current file. Removing a declaration therefore never triggers worktree deletion. Changing a declaration that conflicts with live Git facts stops before mutation.

### Use a dedicated structured diagnostic

`CONFIG_EDIT_REQUIRED` is a configuration-exit diagnostic attached to the task and, when applicable, the repository arguments. Its message tells the caller to edit `<task-root>/taskflow.yaml` and rerun `create` without `--repo`. This makes the correction deterministic for AI callers instead of silently ignoring or merging arguments.

### Update the agent contract with the implementation

The bundled skill and README will document two paths: bootstrap with `create --repo`, then direct YAML edits followed by `create --dry-run` and explicit `create --execute`. Active specs and tests will remove append-through-create scenarios. Archived historical artifacts remain unchanged.

## Risks / Trade-offs

- [An AI agent may expect `create --repo` to add one repository to a live task] → return a stable diagnostic with the exact configuration path and corrective command shape.
- [Direct YAML edits can be malformed] → retain strict decoding and full preflight before any Git mutation; JSON diagnostics remain the machine-facing repair loop.
- [Removing a repository from YAML leaves an unused worktree] → this is intentional non-destructive behavior; cleanup remains outside Taskflow.
- [Existing automation using append semantics breaks] → document the breaking boundary and do not add a compatibility migration layer.

## Migration Plan

1. Change create resolution and its diagnostic contract.
2. Replace append tests with existing-task rejection and direct-YAML reconciliation tests.
3. Update README, bundled skill, active specs, and add the OpenSpec change to the repository history.
4. Run strict OpenSpec validation and the complete Go verification suite.

Rollback is a whole-binary rollback. The new binary does not rewrite or delete existing task files.

## Open Questions

None. `create --repo` is bootstrap-only; existing task topology is edited through `taskflow.yaml`.
