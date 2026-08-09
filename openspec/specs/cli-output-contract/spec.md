## Purpose

Define stable human and machine output and exit-code behavior for the CLI.

## Requirements

### Requirement: Emit a stable machine-readable result envelope
Commands supporting `--json` MUST emit valid JSON with `schemaVersion`, `command`, `ok`, `taskID` when known, `data`, `warnings`, and structured `errors`. JSON output MUST contain no ANSI control sequences.

#### Scenario: Render a configuration failure as JSON
- **WHEN** a JSON-mode command encounters invalid configuration
- **THEN** it emits a result envelope with `ok: false` and at least one structured error code, message, and optional repository and hint

### Requirement: Map expected failures to stable exit codes
The CLI MUST use exit code 0 for success, 1 for execution failure including a launched child process that exits non-zero, 2 for argument or configuration errors, 3 for environment-preflight failure, 4 for partial completion after mutation begins, 5 for deterministic worktree/branch, task-lock, or source-branch-lock conflict, 6 for external-tool incompatibility, and 7 for validation failure. Child process exit values MUST be reported as structured data and MUST NOT replace the stable specflow exit code.

#### Scenario: Return a lock conflict exit code
- **WHEN** an initialization command cannot acquire its task lock
- **THEN** it reports a lock conflict and exits with code 5

#### Scenario: Return a source-branch lock conflict exit code
- **WHEN** another local task holds the same source-repository branch lock
- **THEN** execute-mode start reports `SOURCE_BRANCH_LOCKED` and exits with code 5 before mutation

### Requirement: Preserve facts across output modes
Text and JSON renderings of a command result MUST express the same success state, warnings, and errors, even though their layouts differ.
