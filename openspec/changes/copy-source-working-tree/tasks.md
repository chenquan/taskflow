## 1. Remove the explicit overlay contract

- [x] 1.1 Remove the `local.paths` domain/config model, repeatable `--local` bootstrap option, overlay-specific validation, and related CLI help while preserving existing `taskflow.yaml` repository configuration behavior.
- [x] 1.2 Remove the per-file overlay discovery, snapshot, hash, plan, report, and ownership code, and retire the superseded `worktree-local-file-overlay` change artifacts from the delivery branch.

## 2. Register a target and copy the source working tree

- [x] 2.1 Extend the Git Worktree creation path to support `git worktree add --no-checkout` while preserving configured base, branch, source common directory, and existing branch-tracking behavior.
- [x] 2.2 Implement a standard-library recursive source-tree copier that copies all source working-directory entries, preserves supported directories/files/symlinks and modes, excludes `.git` entries at the source root and at any nested depth, rejects source and target paths that contain one another, and fails with a structured diagnostic for unsupported entries or symlink creation failure.
- [x] 2.3 Populate a new target's index from HEAD with a mixed reset (or equivalent) immediately after registration, then run the copier so the target contains the source working-tree snapshot with tracked modifications appearing as normal unstaged changes; treat index-population failure as a retryable pending-copy failure.
- [x] 2.4 Add one repository-level pending/complete source-copy marker to the existing ownership manifest and persist it atomically before Worktree mutation and after successful copying.

## 3. Reconcile, retry, and report the simplified lifecycle

- [x] 3.1 Update create planning to report a source-copy action for new targets and a repair action for pending owned targets, while keeping dry-run mutation-free and matching manual Worktrees untouched.
- [x] 3.2 Update reconciliation to reuse completed targets without copying, retry pending owned targets by re-copying registered ones and registering missing ones before copying, and retain existing mismatched-target, lock, and branch-conflict behavior.
- [x] 3.3 Update text/JSON output and stable diagnostics for source-copy success, partial failure, index-population failure, unsupported entries, and pending repair, including copied entry and byte totals when available.
- [x] 3.4 Preserve deletion ownership and dirty-worktree safety gates for targets that contain a complete source snapshot; ensure delete never copies, synchronizes, or deletes source files independently.

## 4. Documentation and verification

- [x] 4.1 Update README, bundled Taskflow Skill, command help, and OpenSpec-facing guidance to describe complete source working-tree copying, `.git` exclusion, ignored-file inclusion, reuse, and pending retry behavior.
- [x] 4.2 Add unit tests for recursive copying, tracked modifications, untracked files, ignored files, nested paths, metadata preservation, `.git` exclusion at the root and at nested depths, symlinks pointing outside the source, symlink/special-entry handling, and mutual source/target containment errors.
- [x] 4.3 Add service and compiled-binary tests for dry-run immutability, `--no-checkout` creation with index population, post-create status semantics (tracked modifications as unstaged changes, no staged deletions, clean status when the source matches the base), complete target contents, manual Worktree reuse, pending-copy retry including a missing pending target, completed-copy reuse, output parity, and deletion safety.
- [x] 4.4 Run `go test ./...`, `go vet ./...`, `go test -race ./...`, `git diff --check`, strict OpenSpec validation, and the configured cross-platform build/test checks; resolve all failures before implementation delivery.
