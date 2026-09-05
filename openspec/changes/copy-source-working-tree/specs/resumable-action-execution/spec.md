## MODIFIED Requirements

### Requirement: Recover from live Git and source-copy facts
Create SHALL recover from an interrupted Worktree or source-copy operation by inspecting current Git Worktrees and the repository-level source-copy status, reusing matching targets, repairing a pending source copy, and creating only missing Worktrees without a general action journal or per-file snapshot.

#### Scenario: Retry after Worktree registration and copy interruption
- **WHEN** a Worktree is registered but complete source copying fails and create is run again after the external fault is fixed
- **THEN** create reuses the matching Worktree, reruns the complete source copy, and marks the source-copy status complete only after success

#### Scenario: Retry after an earlier repository succeeds
- **WHEN** a multi-repository create fails after one repository has a complete source copy
- **THEN** a later create reuses the completed repository and repairs or creates only the remaining repositories
