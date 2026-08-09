## MODIFIED Requirements

### Requirement: Diagnose required tool and repository readiness
The CLI SHALL provide `specflow doctor <task-id>` that inspects Git, OpenSpec when change creation is enabled, enabled development-tool executables, each configured source repository, configured base reference, OpenSpec initialization, worktree target safety, dependency graph, and configured-check executable availability. OpenSpec MUST resolve to a parseable semantic version in the supported range `>=1.4.1, <2.0.0`; an unsupported or malformed OpenSpec version MUST be reported as tool incompatibility. Doctor MUST not fetch, create, delete, or otherwise mutate Git or OpenSpec state.

#### Scenario: Report a missing base reference
- **WHEN** a configured repository base reference cannot be found locally
- **THEN** doctor reports a `BASE_REF_NOT_FOUND` diagnostic with a corrective hint and a failing environment status

#### Scenario: Report a dirty source checkout as a warning
- **WHEN** a configured source repository has uncommitted changes
- **THEN** doctor emits a warning while continuing its remaining read-only checks

#### Scenario: Reject unsupported OpenSpec
- **WHEN** change creation is enabled and OpenSpec reports a version outside the supported range
- **THEN** doctor returns a structured tool-compatibility diagnostic with exit code 6
