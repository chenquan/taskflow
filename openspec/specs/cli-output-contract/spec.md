## Purpose

Define stable human and machine output and exit-code behavior for the CLI.
## Requirements
### Requirement: Emit a stable machine-readable result envelope
Commands supporting `--json` MUST emit valid JSON with `schemaVersion`, `command`, `ok`, `taskID` when known, `data`, `warnings`, and structured `errors`. JSON output MUST contain no ANSI control sequences. The supported operational commands are create, open, and delete.

#### Scenario: Render a configuration failure as JSON
- **WHEN** create, open, or delete encounters invalid configuration
- **THEN** it emits a result envelope with `ok: false` and at least one structured error code, message, and optional repository and hint

#### Scenario: Render create action facts as JSON
- **WHEN** create runs in dry-run or execute mode
- **THEN** its data identifies the resolved configuration and each repository's create, reuse, or conflict action

#### Scenario: Render delete action facts as JSON
- **WHEN** delete runs in dry-run or execute mode
- **THEN** its data identifies each owned worktree removal, local branch deletion, and task-directory cleanup action

### Requirement: Map expected failures to stable exit codes
The CLI MUST use exit code 0 for success, 1 for execution failure including a launched child process that exits non-zero, 2 for argument or configuration errors, 3 for environment-preflight failure, 4 for partial completion after Git mutation begins, 5 for deterministic worktree/branch, task-lock, or source-branch-lock conflict, and 6 for external-tool incompatibility. Validation failure is not a supported command result.

#### Scenario: Return a lock conflict exit code
- **WHEN** create cannot acquire its task lock
- **THEN** it reports a lock conflict and exits with code 5

#### Scenario: Return a source-branch lock conflict exit code
- **WHEN** another local task holds the same source-repository branch lock
- **THEN** execute-mode create reports `SOURCE_BRANCH_LOCKED` and exits with code 5 before mutation

### Requirement: Preserve facts across output modes
Text and JSON renderings of a create, open, or delete result MUST express the same success state, warnings, errors, and operational action facts, even though their layouts differ.

#### Scenario: Render equivalent command outcomes
- **WHEN** create, open, or delete is rendered in text mode and JSON mode
- **THEN** both renderings express the same success state, warnings, structured errors, and action data
