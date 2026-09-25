## Purpose

Define path containment, process streaming, managed-worktree inspection, and state-free create safety.
## Requirements
### Requirement: Contain task paths
The CLI MUST reject task IDs that are not a single safe path component before resolving configuration or writing task files.

#### Scenario: Reject traversal task ID
- **WHEN** a user passes `../other-task` to create, delete, or any task-loading command
- **THEN** the command returns a configuration error and writes no file outside the tasks root

### Requirement: Validate and report actual managed repositories
Create MUST inspect configured sources and targets as real Git worktrees, report whether each target will be created or reused, and surface current worktree identity conflicts. Dirty worktrees MUST not be rejected solely for being dirty.

#### Scenario: Dirty managed worktree
- **WHEN** a configured worktree has an uncommitted file while source and branch identity remain valid
- **THEN** create reports the target as reusable

### Requirement: Honor fetch and failure state
Create MUST use locally resolvable base refs, MUST not fetch implicitly, and MUST report a partial execution failure without recording a persistent failure state. A later create MUST reconcile from live Git facts.

#### Scenario: Missing local base
- **WHEN** a configured base ref cannot be resolved locally
- **THEN** create returns `BASE_REF_NOT_FOUND` before taskflow.yaml or Git mutation

#### Scenario: Retry after partial failure
- **WHEN** a worktree creation fails after another repository was created
- **THEN** create reports the failed repository and a later invocation reuses the existing matching worktree
