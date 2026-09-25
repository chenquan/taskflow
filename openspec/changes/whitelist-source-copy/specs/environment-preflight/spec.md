## MODIFIED Requirements

### Requirement: Preflight create reconciliation
Create MUST inspect every configured source, base ref, branch occupancy, target path, worktree identity, and `.worktreeinclude` manifest before taskflow.yaml or Git mutation. Preflight MUST verify that each source is readable, that its manifest exists and passes syntax validation, and that source and target do not contain one another. A missing manifest MUST return `SOURCE_COPY_MANIFEST_MISSING` and an invalid manifest MUST return `SOURCE_COPY_MANIFEST_INVALID`, both before any configuration or Git mutation.

#### Scenario: Preflight all repositories
- **WHEN** any configured repository is not ready
- **THEN** create returns a repository diagnostic before changing configuration or Git state

#### Scenario: Preflight rejects a missing manifest before mutation
- **WHEN** a source repository has no `.worktreeinclude` and create runs in dry-run or execute mode
- **THEN** create returns `SOURCE_COPY_MANIFEST_MISSING` for that repository and taskflow.yaml, worktrees, and branches are unchanged
