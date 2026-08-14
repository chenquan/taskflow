# e2e-command-flow Specification

## Purpose

Define complete Cobra and compiled-binary create/open/delete verification.

## Requirements

### Requirement: Exercise the complete CLI task lifecycle
The test suite SHALL execute the reduced user-facing flow through the Cobra command surface and SHALL verify each command's result, configuration, and relevant Git side effects.

#### Scenario: In-process create and open preparation succeeds
- **WHEN** a test runs create --dry-run, create --execute, and open preflight against temporary valid repositories
- **THEN** dry-run reports actions without creating configuration or worktrees, execute writes taskflow.yaml and ownership metadata and creates worktrees, and open reaches the tool launch boundary

#### Scenario: Incomplete or dirty work is handled from live facts
- **WHEN** a configured target is missing or mismatched, or a matching worktree is dirty
- **THEN** open rejects the missing or mismatched target and accepts the dirty matching target

#### Scenario: Dry-run preserves all managed state
- **WHEN** create dry-run is executed for a new or existing task
- **THEN** taskflow.yaml, lock artifacts, worktrees, branches, source checkout, and target directories remain unchanged

#### Scenario: Repeated execute is idempotent
- **WHEN** create --execute is run twice for the same task
- **THEN** the second command succeeds, reuses every matching worktree, and creates no duplicate branch or directory

#### Scenario: Direct configuration edits reconcile safely
- **WHEN** an existing taskflow.yaml is edited to add a repository and create is run without --repo
- **THEN** create preserves the existing configuration, creates only the newly declared missing worktree, and does not delete any unlisted worktree

#### Scenario: Repository arguments on existing tasks are rejected
- **WHEN** an existing task receives a repository argument through create
- **THEN** the command returns CONFIG_EDIT_REQUIRED and leaves taskflow.yaml and Git state unchanged

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
The test suite SHALL invoke a built `taskflow` binary in addition to in-process Cobra commands and SHALL verify stable JSON/text output and process exit codes for create, open, and delete.

#### Scenario: Subprocess reports a configuration failure
- **WHEN** the built binary runs create or open with an invalid task identifier in JSON mode
- **THEN** the process exits with the configuration exit code and emits a parseable result envelope with `ok: false` and a structured diagnostic

#### Scenario: Successful output preserves command data
- **WHEN** create or open runs without --json
- **THEN** its text output includes the action or launch data needed to understand the result

#### Scenario: Subprocess reduced flow succeeds
- **WHEN** the built binary executes create dry-run and execute against temporary repositories
- **THEN** its process exit codes, JSON envelopes, text reports, and Git worktree state match the create/open/delete contract
