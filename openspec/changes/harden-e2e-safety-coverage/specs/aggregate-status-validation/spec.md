## ADDED Requirements

### Requirement: Exercise validation and readiness blockers end to end
The command-surface test suite SHALL cover configured-check failures and timeouts, stale validation reports, changed worktree heads, dirty worktrees, and missing validation reports through `validate` and `finish --dry-run`.

#### Scenario: Failed check blocks finish
- **WHEN** a configured check exits non-zero for a managed worktree
- **THEN** validation persists a failed report and finish returns the validation exit code without archive or cleanup mutation
