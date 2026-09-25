## MODIFIED Requirements

### Requirement: Render create action facts as JSON
Commands supporting `--json` MUST emit valid JSON with `schemaVersion`, `command`, `ok`, `taskID` when known, `data`, `warnings`, and structured `errors`. JSON output MUST contain no ANSI control sequences. The supported operational commands are create, open, and delete.

#### Scenario: Render a configuration failure as JSON
- **WHEN** create, open, or delete encounters invalid configuration
- **THEN** it emits a result envelope with `ok: false` and at least one structured error code, message, and optional repository and hint

#### Scenario: Render create action facts as JSON
- **WHEN** create runs in dry-run or execute mode
- **THEN** its data identifies the resolved configuration and, for each repository, the worktree action, the source-copy action with its `.worktreeinclude` pattern count, and after execute the copied entry and byte totals

#### Scenario: Render an unmatched-pattern warning
- **WHEN** execute reports a literal `.worktreeinclude` pattern that matched no source path
- **THEN** the warning appears in both text and JSON renderings with the pattern and repository name

#### Scenario: Render delete action facts as JSON
- **WHEN** delete runs in dry-run or execute mode
- **THEN** its data identifies each owned worktree removal, local branch deletion, and task-directory cleanup action
