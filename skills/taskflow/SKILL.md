---
name: taskflow
description: Safely operate Taskflow AI coding workspaces across multiple Git repositories. Use when preparing a task for Codex or Claude Code, checking configuration or environment, planning or creating managed worktrees, opening an AI coding tool, inspecting task state, running configured validation, or producing a pre-completion readiness report with the `taskflow` CLI.
---

# Taskflow

Use the CLI as the authority for AI coding workspace, worktree, and tool-launch operations. Do not replace it with shell-composed Git or filesystem mutations.

## Locate and inspect

1. Ask for the task ID. If no task workspace root is supplied, Taskflow uses the current working directory; pass `--tasks-root` only when the task root is elsewhere.
2. Read `taskflow.yaml` and `.taskflow/inventory.json` inside the task root before choosing an operation.
3. Prefer machine-readable output when another agent, script, or structured diagnosis will consume the result:

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
3. Before worktree creation, run the dry-run and resolve any reported errors:

   ```bash
   taskflow --tasks-root <tasks-root> start <task-id> --dry-run
   ```

4. Present the dry-run actions and obtain explicit user approval before this mutating command:

   ```bash
   taskflow --tasks-root <tasks-root> start <task-id> --execute
   ```

## Work and verify

1. Start one configured development tool from the managed task only:

   ```bash
   taskflow --tasks-root <tasks-root> open <task-id> --tool codex
   taskflow --tasks-root <tasks-root> open <task-id> --tool claude
   ```

   Do not add permission-bypass flags or create nested worktrees.

2. Inspect progress with `status`; execute repository checks with `validate`. To limit validation, use `validate <task-id> --repo <repo-name>`.
3. Before declaring work complete, run `status` and `validate`, then report any blockers. Taskflow does not commit, push, create a PR, merge, or clean worktrees.

## Safety rules

- Keep `--dry-run` and `--execute` mutually exclusive; never infer approval for `--execute`.
- Preserve existing user files and source-checkout changes. Explain conflicts or lock failures instead of deleting or overwriting data.
- Treat commit, push, pull, merge, archive, and worktree cleanup as separate user-authorized actions; Taskflow does not perform them automatically.
- For a failed CLI command, report its diagnostic code and message, then propose the smallest safe corrective action.
