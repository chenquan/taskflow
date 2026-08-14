## REMOVED Requirements

### Requirement: Persist action outcomes and resume safely
**Reason**: A persistent action journal and lifecycle phase are unnecessary when Git worktree facts are authoritative.
**Migration**: Rerun `create`; matching worktrees are reused and missing worktrees are created.

### Requirement: Accept authorized configuration advancement
**Reason**: Config digest advancement through repo add is removed with stateful action execution.
**Migration**: The current taskflow.yaml is the desired configuration; create preflights and atomically persists append changes.

## ADDED Requirements

### Requirement: Recover from live Git facts
Create SHALL recover from an interrupted worktree operation by inspecting current Git worktrees, reusing matching targets, and creating only missing targets without a persistent action journal.

#### Scenario: Retry after interruption
- **WHEN** create partially creates worktrees and is run again after the external fault is fixed
- **THEN** matching worktrees are reused and missing worktrees are created
