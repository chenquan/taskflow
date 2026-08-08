## Context

The current lifecycle code has a task-local lock but no coordination between two task roots that select the same source branch. Configuration loading also launches Git directly, while the application otherwise uses injectable adapters. OpenSpec is only checked for a pathname before start writes state, and child environment overlays can duplicate keys.

## Goals / Non-Goals

**Goals:**

- Keep configuration parsing deterministic and process-free while preserving `config validate` Git-worktree checks.
- Fail execute mode before any mutation when OpenSpec is missing or outside `>=1.4.1, <2.0.0`.
- Serialize local starts for the same canonical Git common directory and branch across task roots.
- Make child environments and dirty-file counts portable and unambiguous.

**Non-Goals:**

- No YAML, state, report, or command-flag migration.
- No distributed or cross-machine lock service.
- No locking for read-only commands, different branches, or unrelated source repositories.

## Decisions

### Keep configuration structure validation pure

`config.Load` and `config.Validate` retain strict YAML, path normalization, dependency, and tool-policy checks, but do not execute Git. An application-level injected Git inspection is used by `config validate`, so its current user-facing guarantee remains while test doubles and command context are available. Moving all source checks to doctor was rejected because it would weaken the existing `config validate` contract.

### Probe OpenSpec once with a semantic version range

The OpenSpec client owns a probe that resolves the executable, runs `--version`, extracts a semantic version, and accepts only major version 1 at least 1.4.1. `doctor` reports the parsed version and start uses the same probe before state persistence. An exact runtime pin was rejected to allow compatible 1.x patch and minor upgrades; CI remains pinned to 1.4.1.

### Lock source branches in Git common metadata

Execute-mode start first acquires its task lock, verifies compatible OpenSpec when needed, then inspects every configured source and acquires an exclusive flock below `<common-dir>/specflow-locks/` before repository preflight. The lock name is a SHA-256 digest of canonical common-dir plus branch. Acquisition is sorted by `(common-dir, branch)` and released in reverse order to avoid deadlock. A held source lock is a deterministic conflict, while lock-directory I/O failures are environment failures. A tasks-root lock directory was rejected because it cannot coordinate separate task roots.

### Normalize process environments and Git status records

The process adapter produces exactly one effective value per environment key, with overlay values winning and Windows key comparison case-insensitive. Git status uses `--porcelain=v1 -z`; dirty record count treats rename and copy pairs as one logical entry. This avoids newline-path ambiguity without changing status output fields.

### Gate snapshot after quality jobs

The snapshot job depends on test, race, and coverage jobs. It remains a non-publishing snapshot build; branch protection remains repository policy.

## Risks / Trade-offs

- [Lock files remain in `.git` after release] → Only empty lock metadata remains under an explicitly namespaced directory; no worktree file or ref is touched.
- [OpenSpec version text changes] → Extract only the first semantic version and classify malformed output as tool incompatibility with fixture coverage.
- [Two starts race after lock acquisition but before Git action] → Shared source+branch locking covers specflow participants; Git remains the final authority for external tools.
- [Pure load defers source Git failures] → `config validate`, doctor, and start preserve explicit source inspection at their existing boundaries.

## Migration Plan

1. Release as an additive runtime behavior change; existing task files remain valid.
2. On the first execute operation, create the namespaced source lock directory if needed.
3. Roll back by reverting code; stale empty lock files are harmless and can be removed manually from `.git/specflow-locks`.

## Open Questions

None.
