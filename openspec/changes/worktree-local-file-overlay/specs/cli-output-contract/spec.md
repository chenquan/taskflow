## MODIFIED Requirements

### Requirement: Emit a stable machine-readable result envelope
Commands supporting `--json` MUST emit valid JSON with `schemaVersion`, `command`, `ok`, `taskID` when known, `data`, `warnings`, and structured `errors`. JSON output MUST contain no ANSI control sequences. The supported operational commands are create and delete, and create data MUST be able to express both worktree and local overlay actions.

#### Scenario: Render a configuration failure as JSON
- **WHEN** create or delete encounters invalid configuration
- **THEN** it emits a result envelope with `ok: false` and at least one structured error code, message, and optional repository and hint

#### Scenario: Render create action facts as JSON
- **WHEN** create runs in dry-run or execute mode for a repository with a local overlay
- **THEN** its data identifies the resolved configuration, each worktree create or reuse action, each overlay copy, repair, skip, or conflict action, and aggregate selected-file facts

#### Scenario: Render delete action facts as JSON
- **WHEN** delete previews or executes an ownership-checked cleanup
- **THEN** its data identifies each owned worktree removal, local branch deletion, and task-directory cleanup action and their results
