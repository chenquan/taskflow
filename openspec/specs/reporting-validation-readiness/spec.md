## Purpose

Define current operational results for create and open without validation history or readiness conclusions.
## Requirements
### Requirement: Report create and open operational results
Create and open SHALL expose their current action, conflict, and launch results through the common text and JSON output contract without validation or readiness history.

#### Scenario: Report a create action
- **WHEN** create previews or executes a repository reconciliation
- **THEN** output includes the repository and its create, reuse, or failure result
