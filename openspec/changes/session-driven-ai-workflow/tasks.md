## 1. Workflow contract and domain model

- [x] 1.1 Define versioned workflow configuration types for linear stages, bounded attempts, check definitions, approval policy, and workflow-level limits.
- [x] 1.2 Implement strict `workflow.yaml` loading, unknown-field rejection, semantic validation, canonical normalization, and configuration digest calculation without changing `taskflow.yaml` behavior.
- [x] 1.3 Define typed runtime models for workflow snapshots, events, attempts, agent reports, check results, leases, operation IDs, and machine-readable errors.
- [x] 1.4 Add valid, invalid, backward-compatibility, and configuration-digest test fixtures covering the workflow-definition requirements.

## 2. Durable runtime and state machine

- [x] 2.1 Implement task-local runtime path resolution under `.taskflow`, including snapshot, JSONL event log, lease, and per-attempt evidence directories.
- [x] 2.2 Implement lock-protected atomic snapshot writes and append-only event writes, with fail-closed handling for malformed or partially written state.
- [x] 2.3 Implement the explicit workflow transition reducer for `ready`, `running`, `verifying`, `awaiting_approval`, `paused`, `unknown`, `needs_attention`, `completed`, and `cancelled`.
- [x] 2.4 Implement attempt and operation idempotency, stale-operation rejection, and configuration-digest checks for all mutating workflow operations.
- [x] 2.5 Implement expiring session leases, renewal, expiry inspection, and safe recovery that marks an interrupted attempt `unknown` until explicitly resolved.
- [x] 2.6 Add runtime tests for concurrent mutation, atomic-write failure, duplicate operations, expired leases, interrupted sessions, configuration changes, and event replay.

## 3. Bounded machine verification

- [x] 3.1 Implement declarative check execution from executable-plus-argv definitions with task/repository cwd restrictions, timeout enforcement, bounded stdout/stderr capture, and an environment allowlist.
- [x] 3.2 Implement stage verification that runs only configured checks, records command metadata and results as attempt evidence, and derives transitions from machine results rather than model claims.
- [x] 3.3 Implement checkpoint report validation for progress, ready, blocked, and approval-needed outcomes, including stage/attempt/session identity checks.
- [x] 3.4 Add verification tests for passing and failing checks, timeout, output limits, invalid cwd, denied environment values, malformed reports, and final-completion requirements.

## 4. Workflow CLI control plane

- [x] 4.1 Add the `taskflow workflow` command group with `--json` output envelopes, stable error codes, bounded operation options, and no Agent-launching behavior.
- [x] 4.2 Implement `workflow validate` and `workflow status`, including config diagnostics, runtime recovery indicators, current stage/attempt, lease state, and latest verification evidence.
- [x] 4.3 Implement `workflow begin` and `workflow checkpoint` with worktree readiness checks, lease acquisition, schema validation, event emission, and idempotent operation handling.
- [x] 4.4 Implement `workflow verify` with configured-check execution, persisted evidence, state transitions, retry-budget accounting, and fail-closed behavior on unverifiable results.
- [x] 4.5 Implement `workflow pause`, `workflow resume`, `workflow approve`, and `workflow cancel` with explicit policy checks, auditable events, and idempotent repeated requests.
- [x] 4.6 Add CLI integration tests proving JSON contract stability, invalid-input handling, stale-session rejection, concurrent mutation protection, and that workflow commands never commit, push, launch Agents, or perform external writes.

## 5. Codex and Claude Skill integration

- [x] 5.1 Add the bundled global `taskflow-workflow` Skill with engine-neutral instructions for one bounded iteration: inspect status, begin or resume, work, checkpoint, verify, and stop on terminal or attention states.
- [x] 5.2 Extend the Skill installer and asset verification to install the same workflow contract into the supported Codex and Claude global Skill locations, with explicit selective-install behavior.
- [x] 5.3 Document the host `/loop` responsibility and ensure the Skill uses `taskflow --json workflow ...` decisions instead of parsing human-readable prose.
- [x] 5.4 Add Skill contract tests and a manual smoke procedure covering active-loop gating, completed/paused/approval no-op behavior, session resume, and the prohibition on nested `codex` or `claude` launches.

## 6. Compatibility, documentation, and release validation

- [x] 6.1 Add an example workflow configuration and document the workflow lifecycle, state model, command reference, evidence layout, retry/approval rules, and recovery procedure.
- [x] 6.2 Update the existing `taskflow` Skill and README to describe optional composition with `taskflow-workflow` while preserving current task initialization, worktree, ownership, and cleanup semantics.
- [x] 6.3 Add compatibility coverage proving repositories without `workflow.yaml` retain the current Taskflow behavior and invalid workflow configuration fails before mutation.
- [x] 6.4 Run focused Go tests, race/concurrency tests, CLI end-to-end tests, Skill asset checks, and `openspec validate --all --strict`; record any environment-dependent manual checks.
