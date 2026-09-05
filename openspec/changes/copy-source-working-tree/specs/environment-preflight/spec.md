## MODIFIED Requirements

### Requirement: Preflight create reconciliation
Create MUST inspect every configured source, base ref, branch occupancy, target path, Worktree identity, and complete-copy target boundary before taskflow.yaml or Git mutation. It MUST verify that a missing target can be registered as a Worktree and that source copying will exclude all source `.git` entries while the configured source and target paths remain disjoint.

#### Scenario: Preflight all repositories
- **WHEN** any configured repository is not ready or its source-copy boundary cannot be established
- **THEN** create returns a repository diagnostic before changing configuration, registering a Worktree, or copying target files
