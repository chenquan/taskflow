---
name: taskflow
description: Safely operate Taskflow AI coding workspaces across multiple Git repositories. Use when initializing or inspecting a task, planning or creating managed worktrees, coordinating repository roles and cross-repository contracts, opening Codex or Claude, recovering a partial start, running configured validation, or reporting readiness with the `taskflow` CLI.
---

# Taskflow

Use the CLI as the authority for task-workspace, worktree, fetch, and tool-launch operations. Do not replace those operations with shell-composed Git or filesystem mutations.

## Next-step guidance

After every taskflow command, report the outcome and state the recommended next step before stopping. Keep it short: a one-line conclusion plus the next command to run. Typical transitions:

- `init` → `start --dry-run` to preview worktree creation and resolve reported errors.
- `start --dry-run` → present the coordination summary, obtain execute approval, then `start --execute`.
- `start --execute` → `open` to start work, or `validate` to run checks.
- `repo add` → `start --dry-run`, then `start --execute` (only the appended repo gets a new worktree).
- `open` → do the work, then `validate`.
- `validate` → `status`, then report any blockers.
- a failed command → report its diagnostic code and message, then propose the smallest safe corrective next command (see Safety rules).

## Locate and inspect

1. Ask for the task ID. The task directory is `<tasks-root>/<task-id>`; `--tasks-root` defaults to the current directory.
2. Before choosing an operation, inspect `taskflow.yaml`, `.taskflow/inventory.json`, `.taskflow/state.json` when present, and the latest validation report when present.
3. Prefer machine-readable output for diagnosis and handoff:

   ```bash
   taskflow --json --tasks-root <tasks-root> status <task-id>
   ```

## Prepare a task

1. Initialize only from explicit local Git repositories:

   ```bash
   taskflow --tasks-root <tasks-root> init <task-id> \
     --primary <repo-name> \
     --repo <repo-name>=<absolute-path>
   ```

2. Treat `init` as metadata-only: it must not create branches, worktrees, commits, or fetch remotes.
3. Before worktree creation, run the dry-run and resolve reported errors:

   ```bash
   taskflow --tasks-root <tasks-root> start <task-id> --dry-run
   ```

4. Before requesting execute approval, present repository roles, dependency order, and the owner of each cross-repository contract. Obtain confirmation of this coordination summary and explicit approval for execution.

   Note that `execution.fetch: true` permits `start --execute` to fetch configured source remotes.

   ```bash
   taskflow --json --tasks-root <tasks-root> start <task-id> --execute
   ```

## Grow a task with another repository

1. To attach another repository after `init` or `start`, use the append-only command. It only updates task metadata and advances the configuration digest; it does not create a worktree:

   ```bash
   taskflow --tasks-root <tasks-root> repo add <task-id> \
     --repo <repo-name>=<absolute-path> \
     [--depends-on <existing-repo>] \
     [--dry-run]
   ```

2. `repo add` is allowed only while the task is in the `initialized`, `started`, or `failed` phase. It never modifies, removes, or reorders existing repositories and never changes the primary repository. The appended repository reuses the `init` defaults and receives no checks.
3. Worktree creation still requires the dry-run and explicit execute flow. After an append, `start --execute` reuses existing worktrees and creates a worktree only for the appended repository:

   ```bash
   taskflow --tasks-root <tasks-root> start <task-id> --dry-run
   taskflow --tasks-root <tasks-root> start <task-id> --execute
   ```

4. After an append, `status` reports `validationStale: true` until the next `validate` regenerates a report for the current configuration. Run `start --execute` to materialize the appended worktree before validating it.

## Work and verify

1. Open a configured tool only after the managed worktrees are ready. Omitting `--tool` selects `development.default_tool`:

   ```bash
   taskflow --tasks-root <tasks-root> open <task-id> --tool codex
   taskflow --tasks-root <tasks-root> open <task-id> --tool claude
   ```

   Pass extra tool arguments after `--` (e.g., `open <task-id> -- --model opus`). Permission-bypass flags are forwarded; nested-worktree flags are rejected whether passed explicitly or forwarded via `--`.

2. Inspect progress with `status`; execute repository checks with `validate`. `validate <task-id> --repo <repo-name>` includes that repository's dependencies.
3. If `start --execute` partially fails, inspect `status` and the persisted state, fix the external cause, then rerun the same execute command. Taskflow reconciles compatible state and actual Git/worktree facts; never delete state or worktrees to force a retry.
4. For `STATE_CONFLICT`, `STATE_INCOMPATIBLE`, lock, or worktree mismatch diagnostics, preserve the existing state and report the diagnostic. Propose the smallest non-destructive correction rather than overwriting files.
5. Before declaring work complete, run `status` and `validate`, then report any blockers. Taskflow does not commit, push, create a PR, merge, or clean worktrees.

## Safety rules

- Keep `--dry-run` and `--execute` mutually exclusive; never infer approval for `--execute`.
- Preserve existing user files and source-checkout changes. Explain conflicts or lock failures instead of deleting or overwriting data.
- Treat commit, push, pull, merge, archive, and worktree cleanup as separate user-authorized actions; Taskflow does not perform them automatically.
- For a failed CLI command, report its diagnostic code and message, then propose the smallest safe corrective action.
