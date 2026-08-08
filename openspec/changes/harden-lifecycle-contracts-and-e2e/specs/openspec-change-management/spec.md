## MODIFIED Requirements

### Requirement: Create an independent OpenSpec change per worktree
When `execution.create_openspec_change` is true, execute mode SHALL invoke OpenSpec in each managed worktree using the configured lower-kebab-case change ID. It MUST treat a pre-existing matching change as complete and MUST stop when the existing change identity is incompatible. When the policy is false, OpenSpec creation, status, and readiness MUST be represented as not configured rather than missing or failed.

#### Scenario: Create changes in dependency order
- **WHEN** change creation is enabled, all required worktrees exist, and OpenSpec is available
- **THEN** start creates one independent change directory in each worktree in repository dependency order

#### Scenario: Reuse an existing change
- **WHEN** the configured change directory already exists in the expected worktree
- **THEN** start reports it as reused and does not create a second change

#### Scenario: Repository opts out of OpenSpec creation
- **WHEN** change creation is disabled for the task
- **THEN** start and readiness operations do not require an OpenSpec executable, directory, status, or tasks file

### Requirement: Validate changes through the OpenSpec JSON contract
For every configured change, validation MUST invoke `openspec status --change <id> --json` and `openspec validate <id> --strict --json --no-interactive`, parse required result fields, and return tool compatibility failure for malformed responses.

#### Scenario: Strict validation rejects invalid artifacts
- **WHEN** tasks are checked but the OpenSpec change fails strict validation
- **THEN** validate reports the affected repository as invalid and does not declare readiness

#### Scenario: OpenSpec returns malformed JSON
- **WHEN** status or strict validation output lacks required JSON fields
- **THEN** the operation returns exit code 6 with a structured compatibility diagnostic
