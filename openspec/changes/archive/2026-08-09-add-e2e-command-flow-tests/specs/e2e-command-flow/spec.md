## ADDED Requirements

### Requirement: Exercise the complete CLI task lifecycle

The test suite SHALL execute the user-facing task lifecycle through the Cobra command surface and SHALL verify each command's result, persisted state, and relevant side effects.

#### Scenario: In-process lifecycle succeeds

- **WHEN** a test runs `init`, `config validate`, `start --dry-run`, `start --execute`, `status`, `validate`, and `finish --dry-run` against a temporary valid repository
- **THEN** every command returns success, the dry-run reports actions without creating a worktree, execute mode creates the configured worktree and OpenSpec change, and the final readiness report is successful after the worktree is committed

#### Scenario: Incomplete or dirty work is rejected

- **WHEN** the managed worktree has unchecked OpenSpec tasks or uncommitted changes
- **THEN** `validate` or `finish --dry-run` returns the validation failure exit code and reports the corresponding blocker

#### Scenario: Dry-run preserves all managed state

- **WHEN** `start --dry-run` is executed for an initialized task
- **THEN** the state file remains byte-for-byte unchanged, no managed worktree or OpenSpec change exists, the source checkout stays clean, and the configured feature branch is not created

#### Scenario: Repeated execute is idempotent

- **WHEN** `start --execute` is run twice for the same task
- **THEN** the second command succeeds without creating another worktree or invoking OpenSpec change creation again

#### Scenario: Diagnose and open the managed task

- **WHEN** `doctor` runs against valid fixtures and `open` launches the configured Codex and Claude tools after start
- **THEN** doctor succeeds, each tool runs in the primary managed worktree with streamed output, and Claude receives its configured additional-instructions environment variable

#### Scenario: Initialization and configuration lifecycle

- **WHEN** a task is initialized twice with equivalent arguments and its normalized configuration is shown and validated
- **THEN** the second initialization reports reuse without overwriting files and both configuration commands expose the expected task, repository, and check definitions

#### Scenario: Start failure resumes safely

- **WHEN** OpenSpec change creation fails after the worktree is created and `start --execute` is retried after the fault is removed
- **THEN** the first command records a failed partial state, the retry reuses the worktree, creates only the missing change, and transitions the task to started

#### Scenario: Concurrent lifecycle operations conflict safely

- **WHEN** one start holds the task lock or one development tool holds the active session lease
- **THEN** a competing command returns the conflict exit code without corrupting state, and the first operation completes after release

#### Scenario: Invalid lifecycle requests preserve state

- **WHEN** mutually exclusive start flags, finish without dry-run, an unsupported tool, an invalid task identifier, an unknown configuration field, or a mismatched worktree target is supplied
- **THEN** the CLI returns the documented structured failure and preserves existing files and managed state

#### Scenario: Finish remains readiness only

- **WHEN** validation succeeds and `finish --dry-run` reports the task ready
- **THEN** the state file, worktree, branch, OpenSpec change, archive state, and cleanup state remain unchanged

### Requirement: Verify executable and rendering boundaries

The test suite SHALL invoke a built `specflow` binary in addition to in-process Cobra commands and SHALL verify stable JSON/text output and process exit codes.

#### Scenario: Subprocess reports a configuration failure

- **WHEN** the built binary runs a command with an invalid task identifier in JSON mode
- **THEN** the process exits with the configuration exit code and emits a parseable result envelope with `ok: false` and a structured diagnostic

#### Scenario: Successful output preserves command data

- **WHEN** lifecycle commands run without `--json`
- **THEN** their text output includes the command result data needed to inspect the dry-run plan or status repository state

#### Scenario: Subprocess lifecycle succeeds

- **WHEN** the built binary executes the complete task lifecycle against temporary repositories
- **THEN** its process exit codes, JSON envelopes, text reports, Git worktree state, and final readiness result match the Cobra command contract
