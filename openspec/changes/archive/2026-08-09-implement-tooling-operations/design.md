## Context

The task configuration already contains tool definitions and repository checks. This phase must preserve the no-shell rule while allowing interactive tool processes to inherit stdio. A task-level session lease is separate from the initialization lock: it remains held for the child process lifetime.

## Goals / Non-Goals

**Goals:**

- Build exact Codex and Claude launch specs from the primary/secondary worktrees.
- Reject dangerous permission-bypass and nested-worktree flags.
- Aggregate current state and run checks in dependency order with stable reports.

**Non-Goals:**

- API calls to Codex/Claude, credential management, archive, cleanup, push, or PR creation.

## Decisions

- Use an adapter interface returning executable, cwd, args, and allowlisted environment overlay. Codex gets repeated `--add-dir`; Claude gets `--add-dir` and optional `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`.
- A lease JSON file under `.specflow/session.json` records PID, tool, start time, primary worktree, and token. Acquire uses a non-blocking lock; a live PID blocks a second session, while dead leases are replaced only while holding the lock.
- `open` invokes `exec.CommandContext` with inherited stdio and returns the child exit code. Tests inspect specs without launching real tools.
- `validate` runs each configured executable with argument arrays and reports failures; no shell strings are accepted. Repositories are ordered topologically, while independent checks may run sequentially in this phase for portability.

## Risks / Trade-offs

- [Interactive process has no TTY in automation] → inherit all standard streams and report the child exit code.
- [Stale session lease] → verify PID liveness and only replace dead leases under the task lock.
- [Long-running check] → honor configured timeout and classify timeout as validation failure.

## Migration Plan

No migration. Existing tasks gain read-only reports and can opt into `open`/`validate`.

## Open Questions

Cross-platform process-group termination remains a later hardening task; phase 3 uses context cancellation and standard process termination.
