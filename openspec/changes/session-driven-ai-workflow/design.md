## Context

Taskflow currently owns a declarative `taskflow.yaml`, safe Git worktree creation and deletion, ownership metadata, locks, dry-run preflight, and installation of the bundled Skill for Codex or Claude. Its current Skill prepares a worktree and tells the user how to launch an Agent; it does not own task progress, Agent sessions, validation, or lifecycle state.

The requested workflow has a different control shape: the user starts `codex` or `claude`, invokes a globally available workflow Skill, and uses the host session's `/loop` facility to keep making progress. The Skill can call the Taskflow CLI, but Taskflow must not start a nested Agent or require an App Server. The session performs reasoning and edits; the CLI persists the workflow contract, checks, evidence, and safe transitions.

The design must support both global Skill targets, preserve all existing worktree behavior, survive a closed or interrupted session, and avoid treating a model's self-report as proof of completion.

## Goals / Non-Goals

**Goals:**

- Provide a local, inspectable, resumable workflow loop controlled from an active Codex or Claude session.
- Keep the Skill engine-neutral while allowing the two hosts' native global and loop mechanisms to invoke it.
- Use a separate strict `workflow.yaml` for objectives, linear stages, bounded checks, retry policy, budgets, and action policy.
- Make Taskflow CLI the source of truth for workflow state, event history, leases, verification results, and terminal decisions.
- Require machine verification before a workflow can become `completed`.
- Make retries, pauses, configuration changes, interrupted attempts, and stale sessions explicit and recoverable.
- Preserve current `taskflow.yaml`, worktree ownership, create/delete safety, and global Skill installation behavior.

**Non-Goals:**

- Starting, embedding, or supervising a Claude/Codex Agent process.
- Implementing Codex App Server, a provider-neutral Agent protocol, or a background daemon.
- Supporting arbitrary workflow DAGs, parallel Agents, multi-tenant hosting, Web UI, or remote execution.
- Automatically committing, pushing, creating/merging PRs, releasing, deleting resources, or writing to external systems.
- Replacing the host's conversation transcript or making global Skill content the source of task-specific state.

## Decisions

### 1. Add a workflow Skill instead of changing the existing workspace Skill

Create a new bundled `taskflow-workflow` Skill and install the same engine-neutral content into both global Skill roots. Keep the existing `taskflow` Skill focused on worktree preparation and cleanup. The new Skill composes with it by requiring a structurally ready worktree before beginning a workflow.

This is additive and avoids contradicting the current Skill contract, which explicitly tells the user to launch the interactive tool themselves. Engine-specific launch syntax belongs to the host CLI; the workflow Skill only calls `taskflow` commands and repository checks from the active session.

### 2. Use the active session as the Agent runtime and `/loop` as the trigger

The workflow Skill defines one bounded iteration. The host's `/loop` repeats a fixed instruction such as “read status, advance one iteration, checkpoint, verify, and stop on terminal or approval state.” The Skill must never recursively launch `codex` or `claude`.

The loop is session-bound. Closing the session stops new iterations, but state and evidence remain on disk; a later session invokes the Skill again and resumes after a recovery check. A future unattended scheduler is a separate change.

### 3. Keep workflow configuration separate from `taskflow.yaml`

`taskflow.yaml` remains the only desired worktree configuration. `workflow.yaml` is a new user-authored contract with its own version and strict decoder. This prevents adding lifecycle fields to the existing worktree schema and allows users to use Taskflow only for worktree setup.

The v1 configuration is a linear ordered list of stages. Each stage has an objective, an attempt limit, and references to named verification checks. Checks use an executable plus argument array, a bounded working directory selected from the task root or configured repository, an optional environment allowlist, and a timeout. Shell-string evaluation is not the default.

### 4. Make CLI state transitions explicit and idempotent

Add a `workflow` command group with these operations:

- `validate` reads and validates both task and workflow configuration plus worktree readiness.
- `status` returns the current snapshot and recent event summary.
- `begin` claims the task, creates an attempt, and returns the active stage and attempt identifier.
- `checkpoint` records a validated Agent report for the active attempt.
- `verify` runs the configured checks and advances, retries, pauses, or completes the workflow.
- `pause`, `resume`, `approve`, and `cancel` implement explicit user control.

Every mutating command takes the task lock, validates the configuration digest and attempt/lease token, appends an event, and atomically replaces the state snapshot. Repeating a command with the same operation identifier must return the original result rather than duplicate a transition.

### 5. Store a snapshot plus an append-only event log

Use the following runtime layout:

```text
.taskflow/
├── workflow-state.json
├── workflow-events.jsonl
├── workflow-lease.json
└── workflow/
    └── attempts/<attempt-id>/
        ├── prompt.md
        ├── report.json
        └── checks/<check-id>.json
```

The snapshot contains schema version, task ID, workflow config digest, current status/stage/iteration, active attempt, session reference when supplied, last verification, approval records, budget counters, and timestamps. The event log is the audit and recovery source. Reports and check outputs are bounded or summarized to prevent unbounded state growth; paths to full user-owned artifacts may be recorded.

Writes use the existing same-directory atomic-write pattern. A corrupt snapshot does not authorize guessing: the CLI preserves the event log, returns a recovery diagnostic, and requires a recovery path before continuing.

### 6. Use an expiring lease because the loop crosses CLI invocations

The existing file lock protects one CLI operation, not an entire interactive session. `begin` therefore creates a workflow lease containing task ID, engine, session reference, owner token, creation time, and expiry. `checkpoint`, `verify`, `pause`, and `resume` must present the owner token or perform the explicit stale-lease recovery flow.

An expired lease permits inspection but not blind continuation. The next session must compare the configured worktrees and recent events, then explicitly resume. A session killed during an attempt produces `unknown` until the worktree and last known operation are inspected.

### 7. Separate Agent reports from machine verification

The Skill asks the Agent to produce a structured checkpoint containing status (`progress`, `ready`, `blocked`, or `needs_approval`), summary, changed paths, commands run, risks, and next action. The CLI validates and records this report but never treats it as completion proof.

`verify` is authoritative for stage completion. It records each check's command, bounded output, exit code, timeout, duration, and result. Only successful required checks may advance the stage or produce `completed`. A model claim that tests passed without corresponding check evidence is rejected.

### 8. Treat high-risk actions as a hard workflow boundary where possible

The Skill only authorizes reading, editing, and local validation. Commit, push, PR, merge, release, deletion, and external writes transition to `awaiting_approval` or remain outside the workflow. `approve` records a human decision and allows the Skill to continue only when the workflow contract permits it.

Because a generic interactive Agent can still issue an arbitrary shell command, Skill text alone cannot guarantee that a user-configured CLI will never run `git push`. The v1 safety guarantee is therefore scoped: the workflow does not instruct or expose those actions, uses isolated worktrees and least-privilege host settings, and does not report them as Taskflow-managed successful actions. Strong pre-execution interception requires a later command proxy, hook, sandbox, or App Server integration.

### 9. Preserve the same workflow contract across Codex and Claude

The bundled source contains one common workflow protocol and small host-neutral examples. `taskflow skill install` installs it to both global targets by default, while the host determines how the user invokes the global Skill and `/loop`. No workflow state or behavior is stored in a provider-specific directory.

The Skill uses `taskflow --json` for all state and verification calls. Text output remains user-readable, but JSON is the integration contract so the model can make decisions from stable codes, statuses, and evidence rather than parsing prose.

### 10. Keep workflow execution foreground and serial in v1

Only one active attempt may exist per task. Stages run in declaration order, and a failed check retries the current stage until its attempt or global iteration budget is exhausted. There is no parallel stage execution or multi-Agent coordination. This makes the lease, evidence, and recovery model deterministic and fits the one active interactive session assumption.

## Risks / Trade-offs

- **[Risk] `/loop` can continue prompting after the task is already complete.** → Every iteration begins with `workflow status`; terminal, paused, approval, unknown, and budget-exhausted states cause a no-op and a clear stop report.
- **[Risk] A killed Agent may have changed files before checkpointing.** → Record `unknown`, preserve the worktree, require recovery inspection, and never replay an ambiguous non-idempotent action automatically.
- **[Risk] Agent reports can be fabricated or incomplete.** → Validate report shape, store it as evidence only, and require independently executed checks for progression.
- **[Risk] Global Skill content can drift from the installed binary.** → Extend the existing installer fingerprint/overwrite behavior and add content tests for both Codex and Claude targets.
- **[Risk] Workflow configuration changes during execution.** → Store a digest in every attempt and fail closed with `CONFIG_CHANGED` until the user explicitly restarts or resumes against the new configuration.
- **[Risk] Verification commands can hang, mutate outside the worktree, or consume excessive resources.** → Use argv-based execution, bounded cwd, timeout, output limits, environment filtering, and per-check/global budgets; document that repository scripts remain trusted inputs.
- **[Risk] CLI-only policy cannot fully intercept arbitrary Agent shell commands.** → Keep external side effects out of the v1 workflow contract and identify command proxy/sandbox integration as a future security-hardening change.
- **[Trade-off] Separate `workflow.yaml` and runtime files add operational surface.** → Keep desired config, runtime state, ownership, and event evidence in clearly separated files with independent versions and strict validation.

## Migration Plan

1. Add the new workflow packages, command namespace, bundled Skill, examples, and tests without changing existing commands or existing `taskflow.yaml` decoding.
2. Install the new Skill globally for both targets only when the user runs the existing Skill installation command; do not modify existing user-installed Skills automatically during ordinary workflow commands.
3. For an existing task, require a valid current `taskflow.yaml` and structurally matching worktrees before allowing `workflow begin`. No automatic migration of old state, inventory, or validation reports is performed.
4. If `workflow.yaml` is absent, the task remains a normal worktree-only Taskflow task. If it is present but invalid, workflow commands fail without modifying worktrees or the desired configuration.
5. Rollback is binary rollback: existing `create`, `delete`, and Skill installation behavior remains usable. Workflow runtime files can be left inert because they are not read by the legacy worktree commands. Do not delete runtime files automatically during rollback.

## Open Questions

There are no blocking v1 questions. The following are explicitly deferred decisions for later changes:

- whether to add a background scheduler for work after the interactive session closes;
- whether to add a command proxy or App Server adapter for enforceable per-tool approvals;
- whether to support parallel stages or multiple Agent sessions;
- whether to add a Web UI or remote task store.
