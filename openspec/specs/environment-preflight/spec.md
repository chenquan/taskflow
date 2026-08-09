## Purpose

Define execute-mode source, fetch, branch, and managed-worktree preflight checks before Taskflow mutations begin.

## Requirements

### Requirement: Preflight execute-mode repository readiness
Execute-mode `start` MUST inspect each configured source repository, worktree target, branch occupancy, and dependency graph before making filesystem or Git mutations. It MUST require a locally resolvable base reference when fetch is disabled, or a usable fetch remote when fetch is enabled.

#### Scenario: Report a missing base reference
- **WHEN** a configured repository base reference cannot be found locally and fetch is disabled
- **THEN** start reports a `BASE_REF_NOT_FOUND` diagnostic before mutation
