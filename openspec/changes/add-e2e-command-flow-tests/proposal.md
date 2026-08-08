## Why

The CLI currently has unit coverage for individual services but no test that exercises the Cobra command surface as a user would. This allows regressions in command wiring, output rendering, exit-code mapping, and interactions between task initialization, worktrees, and OpenSpec state to pass unnoticed.

## What Changes

- Add an end-to-end command-flow test suite covering the normal task lifecycle.
- Exercise both in-process Cobra commands and a built `specflow` binary.
- Use real Git repositories and a deterministic temporary OpenSpec executable fixture.
- Assert filesystem, worktree, OpenSpec, output, and exit-code behavior.
- Cover the complete lifecycle including idempotent initialization, configuration inspection, failure recovery, active-session reporting, conflicts, and readiness-only finish behavior.

## Capabilities

### New Capabilities

- `e2e-command-flow`: Verify complete CLI command flows through Cobra and the executable boundary.

### Modified Capabilities

- None.

## Impact

- Adds Go tests and controllable failure/blocking fixtures under `cmd` without changing production APIs.
- Tests invoke local `git` and a temporary OpenSpec fixture; no new runtime dependency is introduced.
