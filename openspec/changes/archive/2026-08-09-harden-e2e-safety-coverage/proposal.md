## Why

Removing the OpenSpec runtime integration also removed most end-to-end regression coverage. The remaining single happy-path test does not prove Specflow's worktree-safety, validation, rendering, locking, or recovery contracts, and the CI coverage gate now fails.

## What Changes

- Restore a portable, OpenSpec-free E2E fixture and safety regression suite using real Git repositories, checks, development-tool fixtures, Cobra, and a compiled binary.
- Cover multi-repository ordering, zero-mutation preflight failures, lock and branch conflicts, validation/finish blockers, JSON and exit-code contracts, and lifecycle idempotency.
- Upgrade the module Go directive to 1.25 so local and CI test toolchains use the supported module graph.
- Restore the CI coverage threshold of 80 percent through executable regression coverage; no production command behavior is changed.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `e2e-command-flow`: Verify the complete OpenSpec-free CLI lifecycle, safety failures, and executable rendering contracts.
- `aggregate-status-validation`: Verify configured-check failure, timeout, stale validation, and non-mutating finish behavior through the CLI surface.
- `resumable-action-execution`: Verify idempotent worktree reuse and start-lock conflict behavior without OpenSpec actions.

## Impact

Updates `cmd` E2E tests and fixture support, `go.mod`, and CI-validated coverage only. The test suite does not create or invoke OpenSpec artifacts or executables.
