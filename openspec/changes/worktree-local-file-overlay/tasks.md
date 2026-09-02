## 1. Configuration and CLI contract

- [x] 1.1 Add the optional per-repository `local.paths` model, YAML/JSON serialization, normalization, and backward-compatible default behavior.
- [x] 1.2 Add repeatable `--local <repository>=<source-relative-path>` bootstrap parsing and persist the declarations for new tasks.
- [x] 1.3 Reject `--local` on existing tasks with `CONFIG_EDIT_REQUIRED`, and validate repository/path association and duplicate declarations.
- [x] 1.4 Extend the plan and report data model with stable worktree and overlay action kinds, file counts, sizes, and overlay diagnostics.

## 2. Git-aware overlay discovery and preflight

- [x] 2.1 Add Git client support for NUL-delimited discovery of selected untracked and explicitly selected ignored paths without changing the index or source worktree.
- [x] 2.2 Add base-tree path inspection and detect exact file collisions and incompatible file/directory ancestor relationships before worktree creation.
- [x] 2.3 Add filesystem overlay resolution that enforces source containment, rejects `.git` paths, missing paths, tracked files, symlinks, and special file types, and recursively handles directories.
- [x] 2.4 Capture deterministic overlay file metadata and content hashes while holding the execute locks, including support for spaces, newlines, and portable mode bits.
- [x] 2.5 Integrate overlay discovery into create dry-run and execute preflight so every failure occurs before taskflow.yaml, ownership, Git, or target-file mutation.

## 3. Ownership metadata and reconciliation

- [x] 3.1 Extend the ownership manifest with backward-compatible overlay snapshots, source/target identity, expected file metadata, and pending/complete status.
- [x] 3.2 Persist pending overlay metadata atomically before the first worktree mutation and update it atomically after successful materialization without adding a general task state journal.
- [x] 3.3 Update reconciliation to distinguish new creation, ordinary reuse, pending-overlay repair, and matching manual worktrees that must not receive an implicit overlay.
- [x] 3.4 Add stale snapshot and ownership validation, including source identity, target identity, policy/path identity, and incompatible manifest diagnostics.

## 4. Safe overlay materialization and recovery

- [x] 4.1 Implement regular-file copying through destination-local temporary files and atomic no-overwrite publication, preserving supported mode bits and never following symlinks.
- [x] 4.2 Verify source hashes and destination state before publication; report source-change and destination-conflict diagnostics without overwriting user files.
- [x] 4.3 Integrate overlay materialization after `git worktree add` for missing targets and update action statuses and partial-completion reporting.
- [x] 4.4 Implement pending-overlay retry that accepts expected files already present, copies only missing files, and marks the snapshot complete only after all files match.
- [x] 4.5 Preserve completed overlays as immutable creation-time snapshots and ensure later `create` calls never refresh or delete their files.

## 5. Deletion, documentation, and bundled guidance

- [x] 5.1 Preserve delete's dirty-worktree safety gate for copied overlay files and add coverage for normal refusal and explicit `--force --execute` cleanup.
- [x] 5.2 Update command help and README examples for `local.paths`, `--local`, dry-run review, ignored-file opt-in, recovery, and deletion behavior.
- [x] 5.3 Update the bundled Taskflow skill to review overlay plans, use direct taskflow.yaml edits for existing tasks, and prohibit shell copy replacements or unsafe overwrites.
- [x] 5.4 Update skill-content assertions and all affected OpenSpec-facing documentation to match the new output and recovery contract.

## 6. Verification and delivery readiness

- [x] 6.1 Add unit tests for configuration validation, NUL-delimited Git paths, nested directories, ignored files, hashes, modes, unsafe file types, and ownership compatibility.
- [x] 6.2 Add service and end-to-end tests for dry-run immutability, bootstrap, base collisions, manual reuse, atomic no-overwrite behavior, partial copy retry, and completed-overlay immutability.
- [x] 6.3 Add compiled-binary tests for text/JSON overlay actions, stable diagnostics, and exit codes, including paths with spaces and newlines.
- [x] 6.4 Run `go test ./...`, `go vet ./...`, `go test -race ./...`, `git diff --check`, and strict OpenSpec validation; resolve any coverage or cross-platform failures.
