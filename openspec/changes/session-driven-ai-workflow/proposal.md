## Why

Taskflow currently prepares isolated Git worktrees and installs a Skill, but the Agent session still has to coordinate progress, validation, retries, and recovery manually. This change adds a session-driven AI workflow so a user can start Claude or Codex, invoke the globally installed workflow Skill, and use the session loop to make bounded progress toward a verifiable result.

The workflow must remain local, inspectable, and compatible with Taskflow's existing worktree safety model. The Skill owns the conversational procedure, while Taskflow CLI owns durable state and deterministic checks; no Agent daemon or App Server is required.

## What Changes

- Add a strict, task-local `workflow.yaml` for objective, linear stages, verification commands, retry limits, budgets, and allowed actions.
- Add persistent workflow state, append-only events, attempt records, configuration digests, and expiring session leases under `.taskflow/`.
- Add `taskflow workflow` commands for validation, status, begin, checkpoint, verify, pause, resume, approval records, and cancellation.
- Add a globally installable `taskflow-workflow` Skill for both Codex and Claude that drives one bounded iteration per loop and calls the Taskflow CLI.
- Require machine-verifiable checks before reporting completion; bound retries and pause on approval, ambiguity, configuration changes, stale ownership, or exhausted budgets.
- Preserve the existing `taskflow` worktree Skill and existing `create`, `delete`, and `skill install` behavior; the new workflow layer is additive.
- Keep commit, push, pull request, merge, release, deletion, and external writes outside automatic Agent actions in the first version.

## Capabilities

### New Capabilities

- `workflow-definition`: Strict task-local workflow configuration, linear stages, checks, policies, and limits.
- `workflow-session-loop`: Global Codex/Claude Skill protocol for `/loop`-driven bounded iterations and Taskflow CLI interaction.
- `workflow-state-runtime`: Durable workflow state, events, attempts, leases, recovery, and idempotent transitions.
- `workflow-cli-control`: User- and Skill-facing commands for workflow inspection, progression, verification, pausing, resuming, and cancellation.

### Modified Capabilities

<!-- No existing requirement changes: the current worktree configuration and taskflow Skill remain backward compatible. -->

## Impact

- Extends the Go CLI and domain/configuration packages with a new `workflow` command namespace and runtime model.
- Adds workflow configuration and runtime files alongside existing `taskflow.yaml` and ownership metadata.
- Extends bundled Skill installation and content tests for both global Codex and Claude targets.
- Adds process execution for configured, bounded verification commands, but does not make Taskflow responsible for launching or embedding Claude/Codex.
- Requires documentation updates describing the distinction between worktree preparation, session loop control, durable workflow state, and manual external side effects.
