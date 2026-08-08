## Context

Specflow's initial slices established the Cobra surface and lifecycle operations, but several implementation shortcuts now conflict with the published specifications. Start writes state before discovering deterministic conflicts, state cannot distinguish completed actions, OpenSpec readiness is inferred by scanning one Markdown token, and finish reruns arbitrary repository checks despite promising to be non-mutating. Configuration also exposes controls that never affected execution, and process fixtures assume POSIX shells.

The repair must preserve the stable result envelope and exit codes, keep state subordinate to real Git/filesystem facts, remain safe across process crashes, and run on Linux, macOS, and Windows.

## Goals / Non-Goals

**Goals:**

- Make intent, planning, execution, state, and reports agree exactly.
- Reject deterministic start conflicts before any managed mutation.
- Record durable action outcomes and resume from inspected facts.
- Use OpenSpec's strict JSON interfaces for validation and status.
- Make finish readiness read-only and based on a fresh validation report.
- Honor configured development-tool policy and executable paths.
- Prove multi-repository and cross-platform behavior in CI.

**Non-Goals:**

- Automatic migration of the old version-1 YAML shape.
- Repository scanning, worksets, commits, pushes, PRs, archives, or cleanup execution.
- Parallel repository mutation or validation.
- Changing the current `--tasks-root` command interface.

## Decisions

### Redefine configuration v1 instead of migrating it

`execution` contains only `fetch` and `create_openspec_change`; the latter is serialized by init and required on load. Removed fields become unknown-field errors. This deliberately exposes stale configurations instead of guessing whether their ignored false values were intentional. A version bump or automatic migration was rejected because the user explicitly chose a breaking direct redefinition.

Development configuration is validated as a closed policy: the default tool is enabled, every enabled tool has a definition and executable, and only direct launch mode is accepted.

### Separate global preflight from mutation

Start builds one stable action list with an initial directory action and conditional fetch/OpenSpec actions. After acquiring the task lock, it inspects all repositories and targets before writing state or invoking a mutating command. Target or branch mismatches return conflict; missing environment/tool capabilities use their existing typed exits. Only unpredictable failures after mutation begins return partial completion.

Preflight does not call fetch. It verifies the configured remote and other local invariants first; fetch remains the first per-repository mutating action.

### Persist typed action outcomes but trust inspected facts

Each repository state stores action outcomes for fetch, worktree, and OpenSpec with `pending`, `completed`, `skipped`, or `failed`, timestamp, and optional error. The task state also records the directory action. Start writes `starting` only after preflight and atomically updates state after every action.

On retry, worktree and change completion are re-derived from Git and filesystem/OpenSpec facts. Stored outcomes are an audit trail, never authorization to skip inspection.

### Use structured OpenSpec status and strict validation

The adapter invokes `openspec status --change <id> --json` and `openspec validate <id> --strict --json --no-interactive`, parses only stable required fields, and keeps raw-compatible summaries internal. A malformed or unsupported response is a tool-compatibility failure. Tasks progress is derived from `tasks.md` for explicit counts, while strict validity comes only from the CLI result.

Repositories with `create_openspec_change: false` have a `notConfigured` OpenSpec state and are not blocked on OpenSpec artifacts.

### Persist fingerprinted validation reports

`validate` runs repositories sequentially in topological order, optionally limited to one repository while including its dependency closure. It stores one atomic report containing a SHA-256 digest of normalized configuration, each selected worktree HEAD, per-check results, OpenSpec status/strict-validation results, and overall success.

`finish --dry-run` never runs configured checks and never writes a report. It requires a successful report whose configuration digest and repository HEADs match current facts, then performs read-only Git and OpenSpec readiness inspection. Missing, failed, or stale reports are blockers. Unpushed branches are warnings and cleanup blockers, not local readiness blockers.

Merge and validation order is dependency-first topological order; archive and cleanup recommendation is the reverse.

### Keep stable specflow exit codes for child tools

The configured executable is launched only when its tool is enabled. A non-zero child exit returns specflow execution code 1, while result data records the real `childExitCode`, tool, and executable. Exact passthrough was rejected because child values collide with the documented 0-7 meanings.

### Replace shell fixtures with a portable helper executable

Tests build one small Go fixture program and copy it under platform-specific executable names for OpenSpec, Codex, Claude, and checks. Behavior is controlled through environment variables and bounded signal files. This preserves real process boundaries without requiring `/bin/sh`.

CI runs standard tests and vet on Linux, macOS, and Windows; race tests run on Linux and macOS. A pinned OpenSpec 1.4.1 job verifies the real JSON adapter. Coverage is enforced by a Go helper, and GoReleaser produces non-published snapshots for supported OS/architecture pairs.

## Risks / Trade-offs

- [Existing generated YAML becomes invalid] → Emit precise unknown/required-field diagnostics and document regeneration; do not mutate user files automatically.
- [Preflight facts can change before execution] → Keep the task lock, recheck each action immediately before mutation, and classify late races as partial failures without cleanup.
- [OpenSpec JSON evolves] → Parse the minimum required fields, retain fixtures for malformed/version variants, and classify incompatibility separately.
- [Validation report becomes stale after a commit] → Fingerprint normalized config and exact HEADs; require validate after the final commit.
- [Checks may modify worktrees] → Only explicit validate runs them; finish never does.
- [Cross-platform process semantics differ] → Use Go fixture binaries, filepath APIs, bounded polling, and OS-specific executable suffixes.

## Migration Plan

1. Synchronize the existing delta specifications to establish the main-spec baseline.
2. Land the strict v1 configuration and update init/tests together; stale files fail closed.
3. Add state fields additively so old state JSON still decodes; the next successful start reconstructs outcomes from facts.
4. Introduce validation reports as a new optional file; finish blocks until validate creates a fresh successful report.
5. Enable CI and snapshot builds after portable fixtures pass locally.

Rollback is a source-code revert. Existing worktrees and OpenSpec changes are never deleted; new state/report fields are additive and ignored by the prior binary.

## Open Questions

None. Configuration breakage, finish validation semantics, child exit behavior, and delivery scope were explicitly selected before implementation.
