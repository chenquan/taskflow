## MODIFIED Requirements

### Requirement: Exercise the complete CLI task lifecycle
The test suite SHALL execute the reduced user-facing flow through the Cobra command surface and SHALL verify each command's result, configuration, and relevant Git side effects.

#### Scenario: In-process create and open preparation succeeds
- **WHEN** a test runs `create --dry-run`, `create --execute`, and open preflight against temporary valid repositories
- **THEN** dry-run reports actions without creating configuration or worktrees, execute writes only taskflow.yaml and creates worktrees, and open reaches the tool launch boundary

#### Scenario: Incomplete or dirty work is handled from live facts
- **WHEN** a configured target is missing or mismatched, or a matching worktree is dirty
- **THEN** open rejects the missing or mismatched target and accepts the dirty matching target

#### Scenario: Dry-run preserves all managed state
- **WHEN** create dry-run is executed for a new or existing task
- **THEN** taskflow.yaml, lock artifacts, worktrees, branches, source checkout, and target directories remain unchanged

#### Scenario: Repeated execute is idempotent
- **WHEN** create --execute is run twice for the same task
- **THEN** the second command succeeds, reuses every matching worktree, and creates no duplicate branch or directory

#### Scenario: Create appends a repository
- **WHEN** an existing task receives one new repository through create
- **THEN** taskflow.yaml preserves the original repositories, adds the new one, and the next reconciliation creates only its missing worktree

#### Scenario: Partial create retries without state
- **WHEN** worktree creation fails after an earlier repository succeeds and the fault is removed
- **THEN** the first command returns a partial failure without state.json and the retry reuses the earlier worktree before creating the remaining one

#### Scenario: Concurrent create conflicts safely
- **WHEN** one create holds the task lock or source branch lock
- **THEN** a competing command returns the conflict exit code without corrupting taskflow.yaml or existing worktrees

#### Scenario: Invalid requests preserve files
- **WHEN** an invalid task identifier, unknown YAML field, unsupported tool, nested-worktree argument, or mismatched target is supplied
- **THEN** the CLI returns the documented structured failure and preserves existing configuration and Git state

### Requirement: Verify executable and rendering boundaries
The test suite SHALL invoke a built `taskflow` binary in addition to in-process Cobra commands and SHALL verify stable JSON/text output and process exit codes for create and open.

#### Scenario: Subprocess reports a configuration failure
- **WHEN** the built binary runs create or open with an invalid task identifier in JSON mode
- **THEN** the process exits with the configuration exit code and emits a parseable result envelope with `ok: false` and a structured diagnostic

#### Scenario: Successful output preserves command data
- **WHEN** create or open runs without `--json`
- **THEN** its text output includes the action or launch data needed to understand the result

#### Scenario: Subprocess reduced flow succeeds
- **WHEN** the built binary executes create dry-run and execute against temporary repositories
- **THEN** its process exit codes, JSON envelopes, text reports, and Git worktree state match the create/open contract
