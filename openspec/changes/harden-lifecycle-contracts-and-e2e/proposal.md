## Why

The completed lifecycle changes pass their current tests but still violate several published contracts: invalid start targets mutate state, action completion is not persisted, validation does not invoke OpenSpec strict validation, configured tool executables and execution flags are ignored, and readiness reports are incomplete. The test suite also depends on POSIX shell fixtures and does not prove multi-repository or cross-platform behavior.

## What Changes

- **BREAKING** Redefine configuration version 1 so `execution` supports only `fetch` and required `create_openspec_change`; reject the previously serialized but unimplemented workset/commit/push/archive/cleanup fields.
- Make start plans complete and conditional, perform a mutation-free global preflight, and persist typed per-action outcomes for reliable recovery.
- Invoke OpenSpec status and strict validation through a structured adapter and persist fingerprinted validation reports.
- Expand status and finish into deterministic aggregate reports; finish consumes a fresh validation report and remains strictly non-mutating.
- Honor enabled development tools and configured executables while preserving the stable specflow exit-code contract and reporting the child exit code as data.
- Add multi-repository, invalid-state, real OpenSpec, adapter-contract, and cross-platform executable-boundary tests.
- Add CI, coverage enforcement, and snapshot release configuration for Linux, macOS, and Windows.

## Capabilities

### New Capabilities

- `cross-platform-quality-gates`: Define portable fixtures, CI validation, coverage gates, and snapshot release builds.

### Modified Capabilities

- `task-configuration-validation`: Make supported execution and development-tool configuration explicit and strict.
- `environment-preflight`: Add version/capability, branch-occupancy, and target-readiness diagnostics.
- `worktree-start`: Require a complete conditional plan and a global mutation-free preflight.
- `resumable-action-execution`: Persist typed per-action outcomes and preserve state on preflight conflicts.
- `openspec-change-management`: Honor the explicit change-creation policy and validate OpenSpec through its JSON interface.
- `development-tool-sessions`: Use configured executables, enforce enabled tools, and expose child exit facts without violating stable exit codes.
- `aggregate-status-validation`: Add structured status, fingerprinted validation reports, scoped validation, and non-mutating finish readiness.
- `cli-output-contract`: Clarify child-process failure reporting under the stable exit-code envelope.
- `e2e-command-flow`: Require multi-repository and cross-platform lifecycle coverage for every corrected contract.

## Impact

This changes the accepted YAML shape and normalized output, state and validation-report JSON, start conflict behavior, OpenSpec adapter calls, status/finish data, tool-launch selection, tests, and CI/release files. Existing version-1 configurations containing removed execution fields will be rejected and must be regenerated or edited; no automatic migration is provided.
