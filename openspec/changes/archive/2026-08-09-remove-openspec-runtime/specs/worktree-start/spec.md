## MODIFIED Requirements

### Requirement: Plan a complete start operation
The CLI SHALL provide `specflow start <task-id> --dry-run` and list directory, fetch (when configured), and worktree actions in dependency order before execution. Dry-run MUST not modify files or Git state and MUST NOT invoke OpenSpec.

#### Scenario: Dry-run three repositories
- **WHEN** a valid task has three repositories and the user requests dry-run
- **THEN** the result lists all planned worktree actions in deterministic dependency order and leaves the sources and task state unchanged

### Requirement: Create safe managed worktrees
Execute mode MUST acquire the task lock, acquire all required source-branch locks, and complete a read-only preflight for every repository before writing state or invoking a mutating command. Preflight MUST verify source identity, base and remote availability, branch occupancy, and target identity. Execute mode MUST create each configured worktree using argument-array Git invocation with the configured branch and base, reject an existing target or branch that does not match the configured repository, and never delete or overwrite it.

#### Scenario: Create a missing worktree
- **WHEN** every repository passes preflight and its target path is absent and branch available
- **THEN** start creates the worktree at the contained target path from the configured base without requiring OpenSpec

#### Scenario: Reject a mismatched target
- **WHEN** the target path exists but is not the expected worktree
- **THEN** start returns conflict code 5 and preserves task state and the existing directory byte-for-byte
