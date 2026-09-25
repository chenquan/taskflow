## MODIFIED Requirements

### Requirement: Do not provide lifecycle reporting or validation
Taskflow MUST NOT provide status or validate commands, persist validation reports, or infer task readiness from historical state. Current create/delete action facts MUST be reported directly by those commands.

#### Scenario: Removed reporting command
- **WHEN** a user invokes status or validate
- **THEN** the CLI rejects the retired command and leaves the task workspace unchanged
