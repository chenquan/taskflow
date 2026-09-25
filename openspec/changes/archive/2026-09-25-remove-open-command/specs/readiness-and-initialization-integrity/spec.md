## MODIFIED Requirements

### Requirement: Require a non-bare Git worktree source
Create preflight SHALL accept a repository source only when Git inspection reports a non-bare worktree.

#### Scenario: Bare repository source
- **WHEN** a configured source is an existing bare Git repository
- **THEN** create rejects it before writing taskflow.yaml or creating a task workspace
