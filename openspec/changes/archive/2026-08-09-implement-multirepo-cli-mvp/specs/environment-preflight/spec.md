## ADDED Requirements

### Requirement: Diagnose required tool and repository readiness
The CLI SHALL provide `specflow doctor <task-id>` that inspects `git`, `openspec`, enabled development-tool executables, each configured source repository, configured base reference, OpenSpec initialization, worktree target safety, dependency graph, and configured-check executable availability. Doctor MUST not fetch, create, delete, or otherwise mutate Git or OpenSpec state.

#### Scenario: Report a missing base reference
- **WHEN** a configured repository base reference cannot be found locally
- **THEN** doctor reports a `BASE_REF_NOT_FOUND` diagnostic with a corrective hint and a failing environment status

#### Scenario: Report a dirty source checkout as a warning
- **WHEN** a configured source repository has uncommitted changes
- **THEN** doctor emits a warning while continuing its remaining read-only checks

### Requirement: Doctor supports repository-scoped diagnostics
The CLI MUST support limiting doctor diagnostics to a named configured repository while still validating task-level configuration needed to interpret that repository.

#### Scenario: Diagnose one repository
- **WHEN** a user requests doctor for one declared repository
- **THEN** the result contains diagnostics for that repository and task-level configuration only
