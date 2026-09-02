## Context

Taskflow currently treats `taskflow.yaml` as the desired worktree configuration and uses `git worktree add` to materialize missing targets. Git checks out the selected base commit, but source-local files outside the index are not part of that operation. The current reconciliation model is deliberately conservative: it performs complete preflight before mutation, reuses a matching worktree by live Git identity, never overwrites an existing path, and retains partial worktrees for a later retry.

The overlay feature must preserve those properties while adding a filesystem operation that has its own failure and recovery boundary. It must also distinguish a Taskflow-created target from a manually created matching worktree, because copying files into the latter would violate the existing reuse contract.

## Goals / Non-Goals

**Goals:**

- Allow each repository declaration to name explicit source-relative local files or directories to materialize in a newly created worktree.
- Support the same declaration during new-task bootstrap through a repeatable CLI option and persist the normalized result in `taskflow.yaml`.
- Make dry-run show the selected files, sizes, and overlay action without changing the source, task directory, or Git state.
- Validate all overlay inputs and base-tree collisions before the first taskflow or Git mutation.
- Copy regular files safely without following symlinks or overwriting target files.
- Persist a compact snapshot and completion marker in the existing ownership manifest so interrupted copies can be repaired without a general task state journal.
- Preserve manual-worktree reuse, user edits, and the existing dirty-worktree deletion gate.

**Non-Goals:**

- Do not copy modified or deleted tracked files, staged changes, commits, branches, Git metadata, or the entire source directory.
- Do not automatically copy every untracked or ignored file. Bulk discovery modes, glob expansion, and automatic secret handling are out of scope for the first version.
- Do not synchronize source changes into a completed worktree; an overlay is a creation-time snapshot, not a live mirror.
- Do not follow symlinks or materialize sockets, devices, FIFOs, or other special files.
- Do not automatically modify a matching manually managed worktree.

## Decisions

### 1. Model local files as an explicit per-repository overlay

Add an optional repository configuration block with exact source-relative paths. A listed path may be a regular file or a directory; directory contents are recursively considered, but only files outside Git's tracked set are eligible for copying.

Conceptually:

```yaml
repositories:
  - name: app
    source: /Users/me/app
    base: origin/main
    branch: feature/demo
    worktree: worktrees/app
    local:
      paths:
        - .env.local
        - config/dev/
        - .claude/settings.local.json
```

The default is an empty overlay. An explicitly listed ignored file is allowed; an ignored file is never included merely because it exists. Exact paths are preferred over glob patterns so the plan is deterministic and reviewable.

**Alternatives considered:**

- Copying all `??` files by default is convenient but can copy accidental build output and makes a task depend on transient source contents.
- Copying all `??` and `!!` files is unsafe for secrets, IDE settings, and large dependency directories.
- A separate shell command would avoid changing create, but would not provide Taskflow's preflight, ownership, or recovery guarantees.

### 2. Add an explicit bootstrap option and keep existing configuration user-owned

The new task bootstrap accepts a repeatable option in the form `--local <repository>=<source-relative-path>`. The option is valid only while constructing a new task together with `--repo` declarations. Taskflow writes the normalized paths into that repository's `local.paths` field. Existing tasks must be edited directly and then reconciled without repository or local bootstrap arguments, matching the current `CONFIG_EDIT_REQUIRED` contract.

**Alternatives considered:**

- Requiring users to create and edit `taskflow.yaml` before the first execute would make the new-task flow cumbersome and would leave no supported way to declare local files in the initial dry-run command.
- A global `--copy-untracked` flag would not express different local files for different repositories and would not persist an auditable selection.

### 3. Use Git-aware discovery plus filesystem validation

Preflight resolves every configured overlay path beneath its repository source, rejects absolute paths, path escapes, `.git` components, missing paths, tracked files, symlinks, and unsupported file types, and recursively enumerates eligible untracked or explicitly ignored regular files. Git's NUL-delimited path output or an equivalent Git-aware query must be used so spaces, newlines, and other valid path characters are preserved.

The selected base tree is also inspected. A local file is rejected if its target path would collide with a tracked file from the base or if a file/directory ancestor relationship makes the checkout impossible. Source tracked modifications are never copied as an implicit overlay.

**Alternatives considered:**

- A pure filesystem walk cannot reliably distinguish tracked files from local files, especially when the source and base are on different revisions.
- Copying first and letting Git fail later would violate the complete-preflight guarantee and could leave an ambiguous partial target.

### 4. Treat overlay materialization as a second action after Git worktree creation

For a missing target, execute first runs the existing `git worktree add` action and then materializes the overlay. The result contains a worktree action and a separate overlay action in stable repository order. A matching worktree is still classified as `reuse`; it is not refreshed merely because the source overlay changed.

If a Taskflow-owned target has a pending overlay snapshot from an interrupted attempt, reconciliation reports a repair action and only completes that pending snapshot. A matching worktree without Taskflow ownership is never modified by overlay logic.

Each regular file is copied through a temporary file in the destination directory and an atomic no-overwrite publish step. The destination is rechecked before publication; an unexpected existing path or changed content is reported as a conflict rather than overwritten.

### 5. Extend ownership metadata with an overlay snapshot instead of adding a task journal

The existing `.taskflow/ownership.json` entry for a Taskflow-created worktree gains optional overlay metadata:

- normalized source-relative file paths;
- file type, size, mode, and content hash captured during execute preflight;
- source, target, and policy identity needed to reject stale repair attempts;
- repository-level `pending` or `complete` materialization status.

The manifest is written atomically before the first worktree mutation with pending records for new overlays. After all files are verified, it is atomically updated to complete. On retry, an existing destination file with the expected hash is accepted as already materialized; a missing file is copied; a different file produces a conflict. A completed overlay is never refreshed, even if the source file later changes or is deleted.

This keeps recovery metadata attached to the resource Taskflow owns and avoids reviving the removed general-purpose state journal. Older ownership entries without overlay metadata remain valid and are treated as having no creation-time overlay.

**Alternatives considered:**

- No manifest would make a partial copy indistinguishable from an ordinary `reuse` and could cause either silent omission or unsafe re-copy.
- A separate task state file would broaden the current state-free lifecycle contract; ownership metadata is the narrowest durable location.
- A temporary complete worktree followed by `git worktree move` would improve transactionality but introduces staging worktrees, move recovery, and cross-platform Git behavior that is disproportionate for this feature.

### 6. Keep deletion conservative

Copied overlay files are ordinary untracked or ignored files in the target. They therefore continue to make the target dirty according to Git. Delete dry-run and execute retain the existing refusal for dirty worktrees unless `--force --execute` is supplied. Delete never uses the overlay manifest to remove individual files or to bypass the dirty-worktree safety gate.

## Risks / Trade-offs

- **[Source changes during execution]** A source file can change after preflight → capture hashes after the execute lock is held and verify the source before each copy; report a partial/conflict result if it changes.
- **[Partial filesystem mutation]** Worktree creation and file copying cannot be one atomic Git/filesystem transaction → use atomic per-file publication and a pending ownership snapshot to make retries explicit and non-overwriting.
- **[Large explicit directories]** A user can select a large local directory → dry-run must report file count and total bytes before approval; no implicit bulk mode is provided.
- **[Secrets in ignored files]** Explicitly selected ignored files may contain credentials → show every selected path in dry-run and require the existing explicit execute approval; never include ignored files implicitly.
- **[Base revision mismatch]** A file untracked in the source may be tracked by the selected base → inspect the base tree during preflight and fail before writing configuration or Git state.
- **[Cross-platform filesystem behavior]** Permissions, symlinks, special files, and path encoding differ by platform → support regular files only, preserve portable mode bits, use NUL-delimited Git paths, and add platform-focused tests.
- **[Existing task compatibility]** Older ownership manifests have no overlay metadata → keep the field optional and do not mutate already matching targets without a pending overlay record.

## Migration Plan

This is backward-compatible for existing configurations because the overlay block is optional and defaults to empty. New tasks can declare paths with `--local`; existing tasks can add the optional block directly to `taskflow.yaml` before creating a missing worktree. Existing matching worktrees and ownership manifests are not rewritten solely to introduce the feature.

The rollout sequence is:

1. Add configuration and bootstrap parsing with an empty default.
2. Add read-only discovery, base collision checks, and dry-run reporting.
3. Add ownership snapshot persistence and safe materialization for missing targets.
4. Add pending-overlay repair and conflict diagnostics.
5. Update delete/readiness guidance, the bundled skill, README, specs, and end-to-end tests.

Rollback is a code/configuration revert. Removing the optional overlay block from a task stops future overlay creation; already copied files remain ordinary worktree files and are protected by the existing dirty-worktree deletion checks.

## Open Questions

- Whether the user-facing field should be named `local.paths` or `overlay.paths`; the behavior and safety contract are independent of the final name.
- Whether a future version should add an explicitly opt-in `visible-untracked` convenience mode; it is intentionally excluded from this proposal.
