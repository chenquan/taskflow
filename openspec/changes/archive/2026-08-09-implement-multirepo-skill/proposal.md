## Why

The CLI now provides deterministic operations, but Codex and Claude need a shared workflow that knows how to inspect a task, validate configuration, and request confirmation before mutations. A thin, tool-neutral skill prevents duplicated orchestration rules.

## What Changes

- Add a shared `openspec-multirepo` skill with references for workflow, ownership, configuration, and compatibility.
- Add thin Codex and Claude entry points that delegate to the shared workflow.
- Document confirmation boundaries and deterministic CLI-only mutations.

## Capabilities

### New Capabilities

- `openspec-multirepo-skill`: Guide AI agents through task discovery, configuration inference, validation, start, apply, and aggregate verification.

### Modified Capabilities

None.

## Impact

Adds `.agents/skills/openspec-multirepo` and `.claude/skills/openspec-multirepo` documentation only; no runtime CLI behavior changes.
