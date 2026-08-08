## MODIFIED Requirements

### Requirement: Diagnose required tool and repository readiness
The CLI SHALL provide `specflow doctor <task-id>` that inspects Git, enabled development-tool executables, each configured source repository, configured base reference, worktree target safety, dependency graph, and configured-check executable availability. Doctor MUST not fetch, create, delete, or otherwise mutate Git state, and MUST NOT require OpenSpec to be installed or source repositories to contain an `openspec/` directory.

#### Scenario: Report a missing base reference
- **WHEN** a configured repository base reference cannot be found locally
- **THEN** doctor reports a `BASE_REF_NOT_FOUND` diagnostic with a corrective hint and a failing environment status

#### Scenario: Report a dirty source checkout as a warning
- **WHEN** a configured source repository has uncommitted changes
- **THEN** doctor emits a warning while continuing its remaining read-only checks
