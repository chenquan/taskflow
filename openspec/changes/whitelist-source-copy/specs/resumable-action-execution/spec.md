## MODIFIED Requirements

### Requirement: Recover from live Git facts
Create SHALL recover from an interrupted worktree operation by inspecting current Git worktrees, reusing matching targets, and creating only missing targets without a persistent action journal. A matching Taskflow-owned target whose recorded source-copy status is pending SHALL be repaired by rerunning the `.taskflowcopy`-driven overlay; a pending target that no longer exists SHALL be re-registered as a worktree before the overlay is retried.

#### Scenario: Retry after interruption
- **WHEN** create partially creates worktrees and is run again after the external fault is fixed
- **THEN** matching worktrees are reused and missing worktrees are created

#### Scenario: Retry an interrupted overlay
- **WHEN** a worktree was registered but its overlay copy was interrupted, leaving a pending source-copy record
- **THEN** the next create reruns the overlay of manifest-matched paths into that target and marks the copy complete
