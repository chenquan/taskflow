## MODIFIED Requirements

### Requirement: Plan a complete start operation
The CLI SHALL provide `specflow start <task-id> --dry-run` and list a stable directory action followed by fetch actions when configured, worktree actions, and OpenSpec change actions when configured, all in deterministic dependency order. Dry-run MUST not modify files, task state, Git state, or OpenSpec state.

#### Scenario: Dry-run three repositories
- **WHEN** a valid task has three repositories and the user requests dry-run
- **THEN** the result lists directory and every configured repository action in deterministic dependency order and leaves the sources and task state unchanged

#### Scenario: OpenSpec creation is disabled
- **WHEN** `execution.create_openspec_change` is false
- **THEN** neither dry-run nor execute includes or invokes an OpenSpec change action

### Requirement: Create safe managed worktrees
Execute mode MUST acquire the task lock and complete a read-only preflight for every repository before writing state or invoking a mutating command. Preflight MUST verify source identity, base and remote availability, branch occupancy, and target identity. Execute mode MUST create each configured worktree using argument-array Git invocation with the configured branch and base, reject an existing target or branch that does not match the configured repository, and never delete or overwrite it.

#### Scenario: Create a missing worktree
- **WHEN** every repository passes preflight, a target path is absent, and its branch is available
- **THEN** start creates the worktree at the contained target path from the configured base

#### Scenario: Reject a mismatched target
- **WHEN** the target path exists but is not the expected worktree
- **THEN** start returns conflict code 5 and preserves task state and the existing directory byte-for-byte

#### Scenario: Later repository fails preflight
- **WHEN** the first repository is valid but a later repository has an occupied branch or mismatched target
- **THEN** start returns before fetching, creating any worktree, invoking OpenSpec, or writing state for every repository
