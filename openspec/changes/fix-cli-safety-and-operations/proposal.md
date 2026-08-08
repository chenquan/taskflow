## Why

Review found a task-root traversal, broken interactive tool execution, and several configuration/runtime behaviors that diverge from the CLI contract. These defects can write outside a managed task, prevent normal Codex/Claude use, and mislead automation.

## What Changes

- Reject unsafe task IDs consistently when initializing or loading a task.
- Correct streaming process execution and propagate tool environment overlays.
- Verify configured sources are Git worktrees during configuration validation.
- Report managed-worktree status, fail blocked finish reports, and honor configured fetch actions.

## Capabilities

### New Capabilities

- `cli-operational-safety`: Enforce task path containment and correct CLI process, status, finish, and fetch behavior.

### Modified Capabilities

None.

## Impact

Updates the application service, runner, configuration validation, Git adapter, planner, and tests while preserving the existing Cobra command interface and YAML schema.
