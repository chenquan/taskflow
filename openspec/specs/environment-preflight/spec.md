## Purpose

Define create-mode source, local-base, branch, and managed-worktree preflight checks before Taskflow mutations begin.
## Requirements
### Requirement: Preflight create reconciliation
Create MUST inspect every configured source, base ref, branch occupancy, target path, and worktree identity before taskflow.yaml or Git mutation.

#### Scenario: Preflight all repositories
- **WHEN** any configured repository is not ready
- **THEN** create returns a repository diagnostic before changing configuration or Git state
