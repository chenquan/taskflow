## 1. Narrow configuration and persistence

- [x] 1.1 Remove task-level development/tool definitions and `--primary`, preserve repository order, and rely on strict current-schema decoding.
- [x] 1.2 Advance task state and validation report schemas to version 2 and require the current state contract.
- [x] 1.3 Remove inventory models and all initialization/repository-append reads and writes.

## 2. Harden built-in tool launch

- [x] 2.1 Reduce the launch adapter to fixed Codex and Claude executables, default Codex, first-repository cwd, additional directories, Claude environment, argument forwarding, and nested-worktree rejection.
- [x] 2.2 Gate `open` on started schema-v2 state and verified source/branch worktree facts before child execution.
- [x] 2.3 Add unit and E2E coverage for built-in tool launch success, forwarded arguments, dirty worktrees, unstarted state, missing/mismatched worktrees, missing executables, and child exit codes.

## 3. Report only observable status

- [x] 3.1 Remove publication, dependency-readiness, and per-repository validation booleans from status models and rendering.
- [x] 3.2 Rename configuration staleness to `validationConfigStale`; only current-schema reports participate in status.
- [x] 3.3 Update status, append, and validation tests for the breaking JSON and schema-v2 contracts.

## 4. Align guidance and specifications

- [x] 4.1 Rewrite README configuration, lifecycle, breaking-release, and non-goal guidance for the narrowed workspace core.
- [x] 4.2 Update the bundled Taskflow skill to remove inventory, primary-selection, ownership, contract, and unsupported policy instructions.
- [x] 4.3 Run strict OpenSpec validation and resolve every delta-spec consistency issue.

## 5. Verify the release boundary

- [x] 5.1 Run unit, Git-backed E2E, vet, race, production coverage, and diff checks.
- [x] 5.2 Inspect the final change for legacy artifacts, public contract drift, accidental worktree mutation, and complete the implementation checklist.
