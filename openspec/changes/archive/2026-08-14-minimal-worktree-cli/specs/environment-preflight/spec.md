## REMOVED Requirements

### Requirement: Preflight execute-mode repository readiness
**Reason**: The old start-specific preflight is replaced by the broader state-free create reconciliation requirement.
**Migration**: Use create; its preflight covers source, local base, branch, target, and worktree identity before mutation.

## ADDED Requirements

### Requirement: Preflight create reconciliation
Create MUST inspect every configured source, base ref, branch occupancy, target path, and worktree identity before taskflow.yaml or Git mutation.

#### Scenario: Preflight all repositories
- **WHEN** any configured repository is not ready
- **THEN** create returns a repository diagnostic before changing configuration or Git state
