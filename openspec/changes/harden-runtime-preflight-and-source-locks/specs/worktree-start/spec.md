## MODIFIED Requirements

### Requirement: Create safe managed worktrees
Execute mode MUST acquire the task lock, verify compatible OpenSpec when change creation is enabled, acquire all required source-branch locks, and complete a read-only preflight for every repository before writing state or invoking a mutating command. Preflight MUST verify source identity, base and remote availability, branch occupancy, and target identity. Execute mode MUST create each configured worktree using argument-array Git invocation with the configured branch and base, reject an existing target or branch that does not match the configured repository, and never delete or overwrite it.

#### Scenario: Create a missing worktree
- **WHEN** every repository passes preflight, a target path is absent, its branch is available, and compatible OpenSpec is present when configured
- **THEN** start creates the worktree at the contained target path from the configured base

#### Scenario: Reject a mismatched target
- **WHEN** the target path exists but is not the expected worktree
- **THEN** start returns conflict code 5 and preserves task state and the existing directory byte-for-byte

#### Scenario: Reject incompatible OpenSpec before mutation
- **WHEN** OpenSpec change creation is enabled and OpenSpec is unavailable or outside the supported version range
- **THEN** execute mode returns exit code 6 before writing task state, fetching, or creating a worktree
