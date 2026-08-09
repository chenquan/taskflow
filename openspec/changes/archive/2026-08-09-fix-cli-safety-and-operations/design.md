## Context

The existing command pipeline accepts task IDs directly into filesystem joins, always uses `Cmd.Output` even for interactive streams, and ignores launch environment overlays. Start also ignores `execution.fetch`; status reads source rather than managed worktrees; and finish masks failed validation with exit zero.

## Goals / Non-Goals

**Goals:** contain all task filesystem access, support interactive launches, validate actual Git sources, faithfully report managed state, and make configured fetch/exit-code behavior deterministic.

**Non-Goals:** change credential handling, add a YAML remote field, or introduce shell execution.

## Decisions

- A task ID is one portable path component: nonempty, not `.`/`..`, not absolute, and contains no platform separator. Validation is shared by init and every load/path resolver.
- Runner uses `Cmd.Run` for streaming commands and `Cmd.Output` only for captured commands. Explicit environment overlays extend `os.Environ`.
- Git source validation uses `git -C <source> rev-parse --is-inside-work-tree`; this accepts linked worktrees.
- Fetch remote derives from the first segment of `base` when it names an existing remote; otherwise it uses `origin`, and missing remote is a start failure before worktree creation.
- Status inspects the managed worktree; finish returns validation failure when blockers exist.

## Risks / Trade-offs

- [Base ref contains a slash but is not a remote ref] → test remote existence and fall back to origin.
- [Streaming command returns nonzero] → preserve the child exit code and forwarded stderr without attempting capture.

## Migration Plan

No data migration. Existing invalid task paths become explicit configuration errors; existing configurations with non-Git sources fail earlier.
