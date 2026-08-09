## ADDED Requirements

### Requirement: Report stale validation results
Status MUST compare the persisted validation report's configuration digest against the current normalized task configuration. When the report digest no longer matches, status MUST report `validationStale: true`, MUST NOT present the stale report as the current `lastValidation` result, and MUST NOT attribute the stale per-repository validation outcomes to the current repositories. The command MUST leave the persisted `validation.json` on disk as historical evidence, and the next `validate` MUST regenerate a report whose digest matches the current configuration.

#### Scenario: Status flags a stale report after append
- **WHEN** a task has a persisted validation report and its configuration changes because `repo add` advanced the digest
- **THEN** status reports `validationStale: true`, omits the stale per-repository validation outcomes, and leaves `validation.json` on disk

#### Scenario: Validate refreshes the report after append
- **WHEN** the user runs `validate` after the append advanced the digest
- **THEN** the regenerated report's digest matches the current configuration and status no longer reports a stale report
