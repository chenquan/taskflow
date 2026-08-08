## MODIFIED Requirements

### Requirement: Validate repository and dependency constraints
The CLI MUST require unique repository names matching the supported name pattern, an existing source directory, a primary repository present in the repository set, worktree paths contained beneath the task's `worktrees` directory, and an acyclic `depends_on` graph whose references resolve to declared repositories. Loading and structural validation MUST NOT launch external commands. `specflow config validate` MUST additionally inspect every configured source through the Git adapter and reject a source that is not an existing Git worktree.

#### Scenario: Reject a dependency cycle
- **WHEN** repositories form a circular dependency through `depends_on`
- **THEN** configuration validation fails with a dependency-cycle diagnostic

#### Scenario: Reject a worktree path escape
- **WHEN** a repository worktree path resolves outside the task `worktrees` directory
- **THEN** configuration validation fails before any filesystem or Git mutation

#### Scenario: Reject a non-Git source through config validate
- **WHEN** a structurally valid configuration references an existing non-Git source directory
- **THEN** `specflow config validate` rejects the source without changing the task workspace
