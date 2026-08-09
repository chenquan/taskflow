## Purpose

Define execute-mode repository preflight checks.

## Requirements

### Requirement: Preflight execute-mode repository readiness
Execute-mode `start` MUST inspect each configured source repository, base reference, worktree target, branch occupancy, and dependency graph before making filesystem or Git mutations.

#### Scenario: Report a missing base reference
- **WHEN** a configured repository base reference cannot be found locally
- **THEN** start reports a `BASE_REF_NOT_FOUND` diagnostic before mutation
