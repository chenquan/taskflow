## MODIFIED Requirements

### Requirement: Aggregate task status
The CLI SHALL provide `status <task-id>` with task phase and per-repository worktree, branch, HEAD, dirty-file count, upstream, ahead, behind, and inspection error facts in text and JSON modes. Status MUST NOT infer publication, dependency readiness, task completion, or current per-repository validation success.

#### Scenario: Status after start
- **WHEN** a task has started managed worktrees
- **THEN** status reports raw Git and worktree facts for every repository without `pushed`, `dependencyReady`, or per-repository `lastValidationOK` fields

### Requirement: Report stale validation results
Status MUST compare a persisted schema-v2 validation report's configuration digest against the current normalized task configuration and expose the result as `validationConfigStale`. Status MUST retain the report as historical `lastValidation` data even when the digest differs, MUST NOT attribute its outcomes to current per-repository status, and MUST leave the report on disk. The next `validate` MUST regenerate a report whose digest matches the current configuration.

#### Scenario: Status flags a stale report after append
- **WHEN** a task has a persisted validation report and `repo add` advances the configuration digest
- **THEN** status reports `validationConfigStale: true`, returns the report only as historical data, and leaves `validation.json` on disk

#### Scenario: Validate refreshes the report after append
- **WHEN** the user runs `validate` after the append advanced the digest
- **THEN** the regenerated report's digest matches the current configuration and status reports `validationConfigStale: false`

## ADDED Requirements

### Requirement: Persist schema-v2 validation reports
Validation MUST write schema-v2 reports containing the task ID, configuration digest, validation scope, completion time, repository HEADs, check outcomes, and aggregate result. A report with another schema or task ID MUST be treated as incompatible historical data and MUST NOT be used as current status.

#### Scenario: Load an incompatible validation report
- **WHEN** status finds a schema-v1 or wrong-task validation report
- **THEN** status omits it from `lastValidation` without modifying the report file
