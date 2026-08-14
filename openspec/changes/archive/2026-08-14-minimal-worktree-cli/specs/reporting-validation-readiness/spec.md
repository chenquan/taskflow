## REMOVED Requirements

### Requirement: Render operational data in text mode
**Reason**: The validation/status operational reporting surface is removed; create/open retain their own action and launch data through the common output contract.
**Migration**: Use create/open text or JSON output.

### Requirement: Validate in dependency order
**Reason**: Repository dependencies and validation checks are removed.
**Migration**: Run checks directly in the desired order outside Taskflow.

### Requirement: Reject invalid check configuration
**Reason**: Check configuration is no longer part of taskflow.yaml.
**Migration**: Remove checks from taskflow.yaml and configure them in repository tooling.

## ADDED Requirements

### Requirement: Report create and open operational results
Create and open SHALL expose their current action, conflict, and launch results through the common text and JSON output contract without validation or readiness history.

#### Scenario: Report a create action
- **WHEN** create previews or executes a repository reconciliation
- **THEN** output includes the repository and its create, reuse, or failure result
