## MODIFIED Requirements

### Requirement: Diagnose required tool and repository readiness
The CLI SHALL provide `specflow doctor <task-id>` that inspects `git`, OpenSpec when change creation is enabled, enabled development-tool executables, supported tool versions/capabilities, each configured source repository, configured base reference, OpenSpec initialization, branch occupancy, worktree target safety, dependency graph, and configured-check executable availability. Doctor MUST not fetch, create, delete, write probe files, or otherwise mutate Git, task state, or OpenSpec state.

#### Scenario: Report a missing base reference
- **WHEN** a configured repository base reference cannot be found locally
- **THEN** doctor reports a `BASE_REF_NOT_FOUND` diagnostic with a corrective hint and a failing environment status

#### Scenario: Report a dirty source checkout as a warning
- **WHEN** a configured source repository has uncommitted changes
- **THEN** doctor emits a warning while continuing its remaining read-only checks

#### Scenario: Report an occupied branch
- **WHEN** a configured branch is already checked out in a different worktree than its configured target
- **THEN** doctor reports a failing branch-occupancy diagnostic without changing either worktree

#### Scenario: Skip OpenSpec probe when disabled
- **WHEN** `execution.create_openspec_change` is false
- **THEN** doctor does not require the OpenSpec executable or repository initialization
