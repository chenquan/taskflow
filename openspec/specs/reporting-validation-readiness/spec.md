## Purpose

Define operational rendering, ordered checks, readiness blockers, and early validation.

## Requirements

### Requirement: Render operational data in text mode
Text command output MUST render calculated result data in addition to success, warnings, and errors.

#### Scenario: Dry-run plan in text mode
- **WHEN** a user runs start dry-run without JSON
- **THEN** the output includes all planned actions

### Requirement: Validate in dependency order
Validation MUST run repository checks in topological dependency order.

#### Scenario: Dependent listed first
- **WHEN** YAML lists a dependent repository before its dependency
- **THEN** validation executes the dependency check first

### Requirement: Reject invalid check configuration
Configuration validation MUST reject non-empty invalid check timeout values.

#### Scenario: Invalid check timeout
- **WHEN** a repository check timeout is not a valid duration
- **THEN** configuration validation fails before start
