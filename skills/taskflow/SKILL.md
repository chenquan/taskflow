---
name: taskflow
description: Safely operate Taskflow multi-repository Git workspaces, including initialization, worktree planning and execution, built-in Codex or Claude launch, recovery, status, and configured validation.
---

# Taskflow

Use `taskflow` as the authority for task workspace, fetch, worktree, repository-append, and built-in tool-launch operations. Do not replace these mutations with shell-composed Git or filesystem commands.

## Locate and inspect

1. Resolve the task directory as `<tasks-root>/<task-id>`; `--tasks-root` defaults to the current directory.
2. Inspect `taskflow.yaml`, `.taskflow/state.json`, current status, and the latest validation report when relevant. Do not require or interpret `inventory.json`.
3. Prefer `taskflow --json --tasks-root <tasks-root> status <task-id>` for diagnosis and handoff.

Repository order is significant: the first repository is the Codex or Claude cwd. `depends_on` describes only start and validation order; do not infer repository roles, ownership, contracts, or delivery readiness.

## Prepare a task

Initialize only from explicit local Git repositories:

```bash
taskflow --tasks-root <tasks-root> init <task-id> \
  --repo <first-name>=<absolute-path> \
  [--repo <additional-name>=<absolute-path>]
```

`init` is metadata-only. Before worktree creation, run:

```bash
taskflow --tasks-root <tasks-root> start <task-id> --dry-run
```

Present repository order, configured dependencies, planned fetches/worktrees, and reported conflicts. Obtain explicit approval before:

```bash
taskflow --json --tasks-root <tasks-root> start <task-id> --execute
```

Never infer approval for execute mode. A configured `execution.fetch: true` authorizes the planned remote fetch.

## Add a repository

Use the append-only command and repeat the dry-run/execute transition:

```bash
taskflow --tasks-root <tasks-root> repo add <task-id> \
  --repo <name>=<absolute-path> \
  [--depends-on <existing-repo>] \
  [--dry-run]

taskflow --tasks-root <tasks-root> start <task-id> --dry-run
taskflow --tasks-root <tasks-root> start <task-id> --execute
```

`repo add` never changes or reorders existing repositories, never creates a worktree itself, and invalidates the current configuration match of any historical validation report.

## Open and verify

Run `open` only after successful `start --execute`:

```bash
taskflow --tasks-root <tasks-root> open <task-id>
taskflow --tasks-root <tasks-root> open <task-id> --tool claude
taskflow --tasks-root <tasks-root> open <task-id> -- --model <model>
```

Codex is the default. Taskflow validates every configured worktree before launch and passes later repositories plus the task root as additional directories. Explicit model and permission arguments after `--` are forwarded unchanged. Never pass nested-worktree arguments.

After implementation, run configured validation and report raw status facts:

```bash
taskflow --tasks-root <tasks-root> validate <task-id>
taskflow --json --tasks-root <tasks-root> status <task-id>
```

`validate --repo <name>` includes the repository's dependency closure. Treat `lastValidation` as historical evidence and `validationConfigStale` only as a configuration-digest comparison.

## Recovery and safety

- After a partial start, inspect status/state, fix the external cause, and rerun the same `start --execute`; do not delete state or worktrees to force recovery.
- Preserve state on `STATE_CONFLICT`, `STATE_INCOMPATIBLE`, lock, branch, or worktree diagnostics and propose the smallest non-destructive correction.
- This release has a breaking configuration/state contract; do not attempt to migrate old task files. Reinitialize tasks for the current schema.
- Commit, pull, push, PR, merge, archive, release, and cleanup are separate user-authorized workflows outside Taskflow.
- After every command, report the concrete result and the smallest safe next command.
