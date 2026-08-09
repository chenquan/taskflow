## MODIFIED Requirements

### Requirement: Load a strict versioned task configuration
The CLI MUST decode `specflow.yaml` with unknown fields rejected and MUST reject unsupported configuration versions. Configuration version 1 `execution` MUST contain only `fetch` and an explicitly serialized `create_openspec_change`; fields for worksets, commits, pushes, archives, or cleanup MUST be rejected. It MUST normalize source paths to absolute paths and preserve task identity separately from derived branch or change names.

#### Scenario: Reject an unknown YAML field
- **WHEN** a task configuration contains an unrecognized field
- **THEN** `specflow config validate` returns a configuration error identifying that field

#### Scenario: Reject a removed execution field
- **WHEN** a version-1 configuration contains `create_workset`, `commit`, `push`, `archive`, or `cleanup`
- **THEN** configuration validation rejects the stale configuration without rewriting it

#### Scenario: Require change-creation intent
- **WHEN** `execution.create_openspec_change` is omitted
- **THEN** configuration validation fails instead of guessing whether OpenSpec changes are desired

### Requirement: Validate development-tool configuration
Configuration validation MUST require the default tool to be enabled, every enabled tool to have a non-empty configured executable and definition, and every configured launch mode to be `direct`.

#### Scenario: Default tool is disabled
- **WHEN** `development.default_tool` is not present in `development.enabled_tools`
- **THEN** configuration validation returns a configuration error before a session is acquired

#### Scenario: Unsupported launch mode
- **WHEN** an enabled tool uses a launch mode other than `direct`
- **THEN** configuration validation rejects the tool definition
