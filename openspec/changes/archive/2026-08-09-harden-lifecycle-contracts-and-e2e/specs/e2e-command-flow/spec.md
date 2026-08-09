## MODIFIED Requirements

### Requirement: Exercise the complete CLI task lifecycle
The test suite SHALL execute the user-facing task lifecycle through fresh Cobra command trees and built binaries against at least three temporary Git repositories with dependencies. It SHALL verify every command result, persisted state, report fingerprint, relevant side effect, and mutation-free invariant for configured OpenSpec and no-OpenSpec modes.

#### Scenario: In-process lifecycle succeeds
- **WHEN** a test runs init, configuration, doctor, complete dry-run, execute, status, scoped/full validate, open, and finish against three dependent repositories
- **THEN** every successful phase exposes dependency-ordered facts, action outcomes, strict OpenSpec results, and a fresh final readiness report

#### Scenario: Incomplete or dirty work is rejected
- **WHEN** a managed worktree has invalid or incomplete OpenSpec artifacts, a failed check, a stale validation fingerprint, or uncommitted changes
- **THEN** validate or finish returns the documented validation or compatibility exit and identifies the affected repository

#### Scenario: Dry-run preserves all managed state
- **WHEN** start dry-run is executed for an initialized multi-repository task
- **THEN** task files remain byte-for-byte unchanged, no worktree/change/branch exists, source checkouts remain clean, and no external mutating call occurs

#### Scenario: Repeated execute is idempotent
- **WHEN** start execute is run twice for the same task
- **THEN** the second command reconciles completed outcomes without duplicate fetch-independent worktrees, branches, or OpenSpec creation

#### Scenario: Diagnose and open the managed task
- **WHEN** doctor probes portable fixtures and open launches configured custom Codex and Claude executables
- **THEN** versions/capabilities, cwd, streams, environment, arguments, child failure data, and lease cleanup match configuration

#### Scenario: Initialization and configuration lifecycle
- **WHEN** a task is initialized twice with equivalent arguments and its normalized configuration is shown and validated
- **THEN** every existing task file is byte-for-byte unchanged and the strict configuration exposes only supported execution fields

#### Scenario: Start failure resumes safely
- **WHEN** a post-mutation OpenSpec action fails and start is retried
- **THEN** typed outcomes identify completed and failed actions, retry reconciles facts, and only unfinished work runs

#### Scenario: Concurrent lifecycle operations conflict safely
- **WHEN** one start holds the task lock or one development tool holds the active session lease
- **THEN** a competing command returns code 5 without corrupting state, and the first operation completes after release

#### Scenario: Invalid lifecycle requests preserve state
- **WHEN** flags, configuration, a disabled tool, task identity, branch occupancy, or worktree target is invalid
- **THEN** the command returns the documented structured failure and preserves task files, Git facts, and existing target content

#### Scenario: Finish remains readiness only
- **WHEN** validation succeeds and finish reports the task ready
- **THEN** finish does not rerun checks and state, validation report, worktrees, branches, changes, archive state, and cleanup state remain unchanged

#### Scenario: OpenSpec creation is intentionally disabled
- **WHEN** a task explicitly disables OpenSpec change creation
- **THEN** start, status, validate, and finish complete without invoking or requiring OpenSpec while still enforcing Git and configured checks

### Requirement: Verify executable and rendering boundaries
The test suite SHALL invoke built `specflow` and portable Go fixture executables on supported operating systems and SHALL verify stable JSON/text output, process exit codes, paths containing spaces and Unicode, and cleanup of blocked processes.

#### Scenario: Subprocess reports a configuration failure
- **WHEN** the built binary runs a command with invalid or stale version-1 configuration in JSON mode
- **THEN** the process exits with code 2 and emits a parseable failed result with a structured diagnostic

#### Scenario: Successful output preserves command data
- **WHEN** lifecycle commands run without JSON
- **THEN** text output contains the same operational facts required from JSON data

#### Scenario: Subprocess lifecycle succeeds
- **WHEN** the built binary executes the complete three-repository lifecycle
- **THEN** process codes, envelopes, reports, Git/OpenSpec facts, and final readiness match the Cobra contract

#### Scenario: Portable fixtures run on every supported OS
- **WHEN** lifecycle tests run on Linux, macOS, or Windows
- **THEN** fixture behavior requires no POSIX shell and uses the platform executable suffix and filepath semantics
