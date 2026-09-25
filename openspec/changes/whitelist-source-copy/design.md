## Context

The current branch replaces explicit `local.paths` overlays with a creation-time complete copy of the source working directory (`internal/fsx.CopyTree`, `git worktree add --no-checkout` plus a mixed index reset). The complete copy carries every ignored file — caches, dependency directories, build output, local secrets — into every new task worktree. On large checkouts that is slow and expansive, and it gives users no auditable handle on what leaves the source directory.

This change supersedes `copy-source-working-tree` (implemented on this branch, not archived): the complete-copy contract is retired in favor of a whitelist manifest that lives in each source repository.

## Goals / Non-Goals

**Goals:**

- Copy exactly the paths declared in a per-repository `.worktreeinclude` whitelist into newly created worktrees; nothing else.
- Keep the manifest versionable and reviewable inside the source repository, with zero taskflow.yaml surface.
- Keep tracked modifications, untracked files, and ignored files all carryable — the whitelist selects paths, not Git classifications.
- Keep the existing dry-run, locking, target-identity, reuse, ownership-manifest, and deletion safety rules unchanged.
- Keep retry semantics: an interrupted copy is repaired by rerunning the overlay.
- Warn when a literal pattern matches nothing, so silent under-copy is visible at create time.

**Non-Goals:**

- No negation (`!`) patterns, no per-pattern options, no ignore-file inheritance in v1.
- No carrying of tracked-file deletions; the overlay is additive on top of a normal base checkout.
- No synchronization after creation; reuse never refreshes copied content.
- No fallback to complete copy when the manifest is absent — a missing manifest fails loudly.
- No general file-synchronization engine or watch mode.

## Decisions

### 1. Overlay onto a normal checkout, not `--no-checkout` plus index rebuild

The complete copy needed `--no-checkout` and a mixed index reset because it overwrites every file. The whitelist has the opposite shape: unlisted tracked files must exist at base content for the worktree to be usable, and a normal `git worktree add` checkout provides exactly that. Execute therefore creates the worktree with a normal checkout and then overlays manifest-matched paths from the source working directory. Listed tracked modifications overwrite their base copies and surface as unstaged changes; listed untracked/ignored files appear as untracked; everything else stays at base. The `--no-checkout` argument and `git.ResetIndex` are removed.

**Alternative considered:** keeping `--no-checkout` and materializing base with `read-tree` + `checkout-index`. Equivalent outcome, more moving parts, no benefit.

### 2. `.worktreeinclude` at the source repository root, required for new worktrees

The manifest is a dotfile in the repo it describes, so it is versioned, reviewed, and shared with the repository — the people who know which local files matter maintain the list next to the code. Taskflow reads it read-only at create time. A missing manifest fails dry-run and execute with `SOURCE_COPY_MANIFEST_MISSING` before any mutation; a manifest containing only comments means "copy nothing" and must be created deliberately. Copy completeness still derives from the repository-level `pending`/`complete` `SourceCopy` status in the ownership manifest, unchanged from the predecessor design.

**Alternatives considered:** a per-repo path list inside taskflow.yaml (rejected: couples task configuration to repository internals and re-introduces the config surface the retired overlay change removed); silently copying nothing when the manifest is missing (rejected: that is precisely the silent-missing failure mode the whitelist shape risks).

### 3. Gitignore-style matching subset, walk-based, additive

Patterns are matched against source-root-relative paths: `#` comments and blank lines are ignored; a trailing `/` restricts to directories; a pattern containing `/` (other than trailing) is anchored to the source root; a pattern without `/` matches any path segment basename at any depth. `*`, `?`, `[...]`, and `**` are supported through a small segment matcher built on `path.Match`; negation is not supported. Execute walks the source tree once (directory listing is cheap), copies entries that match or live inside a matched directory, creates missing parent directories, and keeps the existing invariants: every `.git` entry (root and nested) excluded, symlinks reproduced without following, unsupported entry types fail with a structured diagnostic naming the path. The overlay never deletes: a tracked file deleted in the source stays at base content in the target.

**Alternatives considered:** full gitignore negation semantics (rejected for v1: re-inclusion inside pruned directories is subtle and hard to audit; whitelist users can list deeper paths explicitly); resolving globs without walking (rejected: pattern-to-tree resolution is significantly more complex for no measurable gain, since copying — not listing — dominates cost).

### 4. Warn on unmatched literal patterns

The inherent whitelist failure is a typo'd pattern that matches nothing. Execute already walks the source tree, so detecting literal (glob-free) patterns that matched no path is free; they are reported as warnings in the result envelope while the copy itself succeeds. Dry-run still does not enumerate source contents — it validates manifest syntax and reports the pattern count only, preserving the cheap dry-run contract.

### 5. Preserve lifecycle, retry, and deletion contracts

`SourceCopy` ownership status, `repair` of pending targets, re-registration of missing pending targets, "matching worktree without a copy record is never populated", dirty-worktree deletion protection, locks, and exit codes are unchanged from the predecessor design.

## Risks / Trade-offs

- **[Silent under-copy]** A pattern that is too narrow leaves needed files behind → missing manifest fails loudly; unmatched literal patterns warn at execute; bundled skill instructs reviewing the manifest and copy actions before execute.
- **[Stale manifest]** Listed paths that no longer exist are skipped without error → same warning surface as unmatched literals keeps drift visible.
- **[Deletions not carried]** A tracked file deleted (uncommitted) in the source reappears at base content in the target → documented additive-overlay semantics.
- **[Large source trees still walked]** Matching walks every entry even when the whitelist is tiny → lstat-only walk; copying dominates cost; optimize only if measured.
- **[Secrets must be excluded deliberately]** The whitelist copies whatever is listed, including sensitive files → the manifest is the explicit, reviewable record of what is carried.
- **[Windows filename restrictions]** Git cannot track some platform-invalid names; the overlay inherits plain filesystem semantics → structured diagnostic on failure, consistent with the predecessor design.

## Migration Plan

1. Replace `CopyTree` usage with manifest parsing plus overlay copy; remove `--no-checkout` and `ResetIndex`.
2. Add preflight manifest validation, `SOURCE_COPY_MANIFEST_MISSING`/`SOURCE_COPY_MANIFEST_INVALID` diagnostics, and unmatched-literal warnings.
3. Update README, bundled skill, delta specs, and e2e tests; retire the superseded `copy-source-working-tree` change artifacts.
4. Existing tasks created by the complete-copy build keep working: their worktrees are matching owned targets with complete copy status and are reused; the manifest is only consulted for new copies.

Rollback is a code revert; worktrees created under the whitelist remain ordinary Git worktrees with dirty overlays.

## Open Questions

None — matching semantics are intentionally a minimal v1 subset; negation and ignore-file inheritance can be added later without breaking the manifest format.
