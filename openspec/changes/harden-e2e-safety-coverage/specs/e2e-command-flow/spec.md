## MODIFIED Requirements

### Requirement: Exercise the complete CLI task lifecycle
The test suite SHALL execute the user-facing task lifecycle through the Cobra command surface and SHALL verify each command's result, persisted state, and relevant Git side effects without requiring or invoking OpenSpec.

#### Scenario: Multi-repository lifecycle succeeds
- **WHEN** a test initializes dependent temporary Git repositories, configures checks, and runs `config validate`, `doctor`, dry-run and execute start, status, scoped validation, full validation, and finish dry-run
- **THEN** commands use deterministic dependency order, create only managed worktrees, report readiness, and leave finish as a non-mutating report

#### Scenario: Safety failures preserve state
- **WHEN** preflight finds a conflicting target, occupied branch, source lock, or invalid command request
- **THEN** the CLI returns its documented structured failure without creating or altering unrelated worktrees, state, or source repositories

#### Scenario: Validation blockers are reported
- **WHEN** a configured check fails or times out, validation becomes stale, or a worktree is dirty
- **THEN** validation or finish returns the validation exit code and identifies the blocking repository without invoking OpenSpec

### Requirement: Verify executable and rendering boundaries
The test suite SHALL invoke a built `specflow` binary in addition to in-process Cobra commands and SHALL verify stable JSON/text output and process exit codes without OpenSpec fields.

#### Scenario: Compiled binary lifecycle succeeds
- **WHEN** the built binary executes a temporary multi-repository task lifecycle
- **THEN** its process exit codes, JSON envelopes, text reports, and Git worktree state match the Cobra command contract and no OpenSpec command is invoked
