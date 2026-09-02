## MODIFIED Requirements

### Requirement: Recover from live Git facts
Create SHALL recover interrupted worktree operations by inspecting current Git worktrees and the optional pending overlay snapshot in ownership metadata. It SHALL reuse matching worktrees, create only missing targets, repair only pending overlay files, and use no general-purpose task action journal. A completed overlay snapshot SHALL be treated as immutable for later creates.

#### Scenario: Retry after partial worktree creation
- **WHEN** create partially creates worktrees and is run again after the external fault is fixed
- **THEN** matching worktrees are reused and missing worktrees are created

#### Scenario: Retry after partial overlay materialization
- **WHEN** a worktree exists with a pending overlay snapshot and some selected files are missing
- **THEN** create verifies expected files, copies only missing files, and marks the overlay complete after all files match

#### Scenario: Preserve a changed destination during retry
- **WHEN** a pending overlay destination exists with content different from its snapshot
- **THEN** create returns a conflict and does not overwrite the destination
