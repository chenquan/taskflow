## MODIFIED Requirements

### Requirement: Map expected failures to stable exit codes
The CLI MUST use exit code 0 for success, 1 for execution failure including a launched child process that exits non-zero, 2 for argument or configuration errors, 3 for environment-preflight failure, 4 for partial completion after mutation begins, 5 for deterministic worktree/branch, task-lock, source-branch-lock, or active-session conflict, and 7 for validation failure. Child process exit values MUST be reported as structured data and MUST NOT replace the stable specflow exit code. OpenSpec compatibility diagnostics and their exit-code mapping MUST NOT be emitted.

#### Scenario: Return a lock conflict exit code
- **WHEN** an initialization command cannot acquire its task lock
- **THEN** it reports a lock conflict and exits with code 5

#### Scenario: Return a source-branch lock conflict exit code
- **WHEN** another local task holds the same source-repository branch lock
- **THEN** execute-mode start reports `SOURCE_BRANCH_LOCKED` and exits with code 5 before mutation
