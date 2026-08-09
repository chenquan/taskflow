## Purpose

Define complete Cobra and compiled-binary lifecycle verification.

## Requirements

### Requirement: Exercise the complete CLI task lifecycle
The test suite SHALL execute the user-facing task lifecycle through the Cobra command surface and SHALL verify each command's result, persisted state, and relevant side effects.

#### Scenario: In-process lifecycle succeeds
- **WHEN** a test runs `init`, `start --dry-run`, `start --execute`, `status`, and `validate` against a temporary valid repository
- **THEN** every command returns success, the dry-run reports actions without creating a worktree, execute mode creates the configured worktree, and validation succeeds after the worktree is prepared

#### Scenario: Incomplete or dirty work is rejected
- **WHEN** a configured repository check fails or times out
- **THEN** `validate` returns the validation failure exit code and reports the corresponding blocker

#### Scenario: Dry-run preserves all managed state
- **WHEN** `start --dry-run` is executed for an initialized task
- **THEN** the state file remains byte-for-byte unchanged, no managed worktree exists, the source checkout stays clean, and the configured feature branch is not created

#### Scenario: Repeated execute is idempotent
- **WHEN** `start --execute` is run twice for the same task
- **THEN** the second command succeeds without creating another worktree and reuses compatible persisted action outcomes

#### Scenario: Initialization and configuration lifecycle
- **WHEN** a task is initialized twice with equivalent arguments
- **THEN** the second initialization reports reuse without overwriting files

#### Scenario: Start failure resumes safely
- **WHEN** fetch or worktree creation fails after an earlier action completed and `start --execute` is retried after the fault is removed
- **THEN** the first command records a failed partial state, the retry reuses compatible completed actions, runs only unfinished work, and transitions the task to started

#### Scenario: Concurrent lifecycle operations conflict safely
- **WHEN** one start holds the task lock
- **THEN** a competing command returns the conflict exit code without corrupting state, and the first operation completes after release

#### Scenario: Invalid lifecycle requests preserve state
- **WHEN** mutually exclusive start flags, an unsupported tool, an invalid task identifier, an unknown configuration field, or a mismatched worktree target is supplied
- **THEN** the CLI returns the documented structured failure and preserves existing files and managed state

### Requirement: Verify executable and rendering boundaries
The test suite SHALL invoke a built `taskflow` binary in addition to in-process Cobra commands and SHALL verify stable JSON/text output and process exit codes.

#### Scenario: Subprocess reports a configuration failure
- **WHEN** the built binary runs a command with an invalid task identifier in JSON mode
- **THEN** the process exits with the configuration exit code and emits a parseable result envelope with `ok: false` and a structured diagnostic

#### Scenario: Successful output preserves command data
- **WHEN** lifecycle commands run without `--json`
- **THEN** their text output includes the command result data needed to inspect the dry-run plan or status repository state

#### Scenario: Subprocess lifecycle succeeds
- **WHEN** the built binary executes the complete task lifecycle against temporary repositories
- **THEN** its process exit codes, JSON envelopes, text reports, and Git worktree state match the Cobra command contract
