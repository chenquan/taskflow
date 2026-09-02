## MODIFIED Requirements

### Requirement: Exercise the complete CLI task lifecycle
The test suite SHALL execute the reduced user-facing create/delete flow through the Cobra command surface and SHALL verify each command's result, configuration, ownership metadata, overlay files, and relevant Git side effects.

#### Scenario: In-process create with an overlay succeeds
- **WHEN** a test runs create --dry-run and create --execute against temporary valid repositories with explicit local overlay paths
- **THEN** dry-run reports worktree and overlay actions without creating configuration or worktrees, and execute writes taskflow.yaml and ownership metadata, creates worktrees, and materializes selected files

#### Scenario: Overlay conflicts are handled from preflight facts
- **WHEN** a selected overlay path is missing, unsafe, or collides with the selected base tree
- **THEN** create rejects the request before configuration, ownership, Git, or target-file mutation

#### Scenario: Dry-run preserves all managed state
- **WHEN** create dry-run is executed for a new or existing task with overlay declarations
- **THEN** taskflow.yaml, lock artifacts, ownership metadata, worktrees, branches, source checkout, and target directories remain unchanged

#### Scenario: Repeated execute is idempotent
- **WHEN** create --execute is run twice for the same task and overlay snapshot
- **THEN** the second command succeeds, reuses every matching worktree and completed overlay, and creates no duplicate branch, directory, or copied file

#### Scenario: Direct configuration edits reconcile safely
- **WHEN** an existing taskflow.yaml is edited to add a repository and local overlay paths and create is run without bootstrap arguments
- **THEN** create preserves the existing configuration, creates only the newly declared missing worktree and overlay, and does not delete any unlisted worktree

#### Scenario: Bootstrap arguments on existing tasks are rejected
- **WHEN** an existing task receives a repository or local bootstrap argument through create
- **THEN** the command returns CONFIG_EDIT_REQUIRED and leaves taskflow.yaml, ownership metadata, overlay files, and Git state unchanged

#### Scenario: Partial overlay creation retries safely
- **WHEN** worktree creation succeeds but overlay materialization fails after one or more files are published and the fault is removed
- **THEN** the first command returns a partial failure with pending ownership metadata and the retry repairs only missing or hash-matching files without overwriting a changed destination

#### Scenario: Concurrent create conflicts safely
- **WHEN** one create holds the task lock or source branch lock
- **THEN** a competing command returns the conflict exit code without corrupting taskflow.yaml, ownership metadata, existing worktrees, or overlay files

#### Scenario: Invalid requests preserve files
- **WHEN** an invalid task identifier, unknown YAML field, invalid overlay path, unsafe overlay file, or mismatched target is supplied
- **THEN** the CLI returns the documented structured failure and preserves existing configuration, ownership metadata, Git state, and target files

### Requirement: Verify executable and rendering boundaries
The test suite SHALL invoke a built `taskflow` binary in addition to in-process Cobra commands and SHALL verify stable JSON/text output and process exit codes for create and delete, including overlay action data.

#### Scenario: Subprocess reports a configuration failure
- **WHEN** the built binary runs create or delete with an invalid task identifier in JSON mode
- **THEN** the process exits with the configuration exit code and emits a parseable result envelope with `ok: false` and a structured diagnostic

#### Scenario: Successful output preserves overlay data
- **WHEN** create runs with or without --json for a task containing local overlay paths
- **THEN** its output includes the worktree and overlay action data needed to understand the result

#### Scenario: Subprocess reduced flow succeeds
- **WHEN** the built binary executes create dry-run and execute against temporary repositories with local overlays
- **THEN** its process exit codes, JSON envelopes, text reports, ownership metadata, copied files, and Git worktree state match the create/delete contract
