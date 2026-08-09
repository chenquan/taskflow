## MODIFIED Requirements

### Requirement: Load a strict versioned task configuration
The CLI MUST decode `specflow.yaml` with unknown fields rejected and MUST reject unsupported configuration versions. It MUST normalize source paths to absolute paths and preserve task identity separately from derived branch names. The loader MUST accept a legacy `execution.create_openspec_change` field when present, ignore its value, and omit it from the active configuration model; all other unknown fields MUST remain rejected.

#### Scenario: Load a configuration with the deprecated creation field
- **WHEN** a version-1 task configuration contains `execution.create_openspec_change`
- **THEN** `specflow config validate` accepts the configuration without making OpenSpec available or changing command behavior

#### Scenario: Reject an unknown YAML field
- **WHEN** a task configuration contains an unrecognized field other than the deprecated creation field
- **THEN** `specflow config validate` returns a configuration error identifying that field

### Requirement: Display normalized configuration
The CLI SHALL provide `specflow config show <task-id>` and `specflow config validate <task-id>` using the same validated configuration model. The show command MUST not mutate the task workspace or source repositories and MUST NOT emit the deprecated OpenSpec creation field.

#### Scenario: Show a valid configuration as JSON
- **WHEN** a user requests `config show` with JSON output for a valid task
- **THEN** the response includes the normalized configuration in the stable result envelope without an OpenSpec creation field
