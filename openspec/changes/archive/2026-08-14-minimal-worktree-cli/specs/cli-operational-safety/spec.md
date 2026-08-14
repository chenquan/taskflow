## MODIFIED Requirements

### Requirement: Contain task paths
The CLI MUST reject task IDs that are not a single safe path component before resolving configuration or writing task files.

#### Scenario: Reject traversal task ID
- **WHEN** a user passes `../other-task` to create, open, or any task-loading command
- **THEN** the command returns a configuration error and writes no file outside the tasks root

### Requirement: Run interactive tools with streams and environment
When command streams are supplied, the runner MUST execute the child without output capture conflicts and MUST apply explicit environment overlays. Non-streaming commands MUST retain captured stdout/stderr behavior.

#### Scenario: Launch Claude with instruction overlay
- **WHEN** Claude additional instructions are enabled
- **THEN** the child process receives `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`

### Requirement: Validate and report actual managed repositories
Create and open MUST inspect configured sources and targets as real Git worktrees. Open MUST report current worktree identity conflicts and create MUST report whether each target will be created or reused. Dirty worktrees MUST not be rejected solely for being dirty.

#### Scenario: Dirty managed worktree
- **WHEN** a configured worktree has an uncommitted file while source and branch identity remain valid
- **THEN** open succeeds and create reports the target as reusable

### Requirement: Honor fetch and failure state
Create MUST use locally resolvable base refs, MUST not fetch implicitly, and MUST report a partial execution failure without recording a persistent failure state. A later create MUST reconcile from live Git facts.

#### Scenario: Missing local base
- **WHEN** a configured base ref cannot be resolved locally
- **THEN** create returns `BASE_REF_NOT_FOUND` before taskflow.yaml or Git mutation

#### Scenario: Retry after partial failure
- **WHEN** a worktree creation fails after another repository was created
- **THEN** create reports the failed repository and a later invocation reuses the existing matching worktree
