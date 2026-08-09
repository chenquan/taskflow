## Context

Taskflow persists three artifacts per task: `taskflow.yaml` (normalized configuration), `.taskflow/inventory.json` (Git facts per repository), and `.taskflow/state.json` (phase, per-repository action outcomes, and a `configDigest` that is the SHA-256 of the normalized configuration). `start --execute` treats `state.configDigest` as a guard: if it differs from the digest of the currently loaded configuration, start returns `STATE_CONFLICT` before mutating anything. `status` renders the latest `validation.json` as the current per-repository validation result.

Because the digest covers the entire configuration, adding a repository by hand invalidates the digest and trips the conflict guard, while a stale `validation.json` keeps reporting outdated results. There is no supported way to grow a task after initialization, so the only options today are destructive edits or recreating the task.

## Goals / Non-Goals

**Goals:**
- Provide an explicit, append-only `repo add` command that grows task metadata without creating worktrees.
- Advance the configuration digest authoritatively so the next `start --execute` reconciles only the appended repository.
- Preserve existing action outcomes, worktrees, and historical validation evidence.
- Surface stale validation reports in `status` without deleting them.

**Non-Goals:**
- Modifying, removing, or reordering existing repositories, or changing the primary repository.
- Allowing multiple repositories per invocation (append one at a time) or appending during a transient `starting` phase.
- Weakening `start`'s digest guard for any path other than `repo add`.
- Committing, pushing, or any source-side mutation beyond what `init` already performs.

## Decisions

### Separate `repo add` from `start`
Worktree creation continues to require the dry-run then explicit-execute flow. `repo add` only mutates metadata and the digest; it acquires the task lock but not source-branch locks, because no worktree is created. This keeps the two concerns independently testable and matches the existing `init`/`start` separation.

**Alternative considered:** Have `repo add` create the worktree immediately. Rejected because it bypasses the dry-run approval gate and conflates metadata mutation with Git mutation.

### Append-only with `init` defaults
The appended repository is constructed with `base: HEAD`, `branch: feature/<task-id>`, `worktree: worktrees/<name>`, no checks, and no dependencies, exactly mirroring `init`. The merged task then runs through the existing `config.Validate`, which enforces name format, worktree containment, dependency references, and acyclicity. This reuses the canonical validation path rather than duplicating rules.

**Alternative considered:** Accept full repository configuration via flags. Rejected as scope creep; users who need custom base, branch, or checks can edit `taskflow.yaml` only before the first `start`, as today.

### Controlled digest migration is the only authorized advancement
`repo add` computes the digest of the merged configuration, writes it into `state.configDigest`, and adds pending actions for the appended repository while leaving existing outcomes untouched. `start`'s existing conflict check then passes for the advanced digest and its resumable loop skips completed repositories and creates only the appended worktree. No `start` code changes are required, which keeps the digest guard meaningful for every other configuration change.

### Phase gating
Append is allowed only for `initialized`, `started`, and `failed`. A persisted `starting` phase means a prior start was interrupted; appending then is ambiguous, so the command rejects it and points the caller at recovering the start first. The task lock serializes `repo add` against concurrent `start` and concurrent appends.

### Atomic snapshot-and-restore writer
Before writing, the command snapshots the current bytes of `taskflow.yaml`, `inventory.json`, and `state.json`. It appends the new repository's Git facts to the loaded inventory (rather than rebuilding it) to minimize work and preserve existing facts. Each file is written with the existing atomic write helper. If any write fails, the writer restores all three files from their snapshots (removing a file only if it did not previously exist) and returns an execution error. All validation runs before the first write, so conflicts never leave a partial mutation.

### Stale validation handling in `status`
`status` computes the current configuration digest and compares it to `validation.json`'s digest. On mismatch it sets `validationStale: true` on the status payload, omits the stale report from `lastValidation`, and does not project stale per-repository outcomes onto the current repositories. The file stays on disk; the next `validate` overwrites it with a digest-matching report.

## Risks / Trade-offs

- [Append during `failed` phase may mask a prior failure] → The command preserves the `failed` phase and existing outcomes; the appended repository simply receives pending actions, and the caller must still rerun `start --execute` to recover, which is the existing recovery path.
- [Two writes succeed before a third fails] → The snapshot-and-restore writer rewrites all three files to their prior bytes on any failure, so the on-disk task returns to its pre-append state.
- [Stale report lingers on disk] → Acceptable and intended: it is historical evidence, clearly flagged as stale by `status` and replaced on the next `validate`.
- [Inventory append vs rebuild] → Appending preserves existing facts and avoids re-inspecting every source; the merged task still passes `config.Validate`, which does not depend on inventory contents.
