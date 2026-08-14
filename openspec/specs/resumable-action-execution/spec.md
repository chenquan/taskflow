## Purpose

Define recovery from interrupted worktree creation using live Git facts without durable action outcomes.
## Requirements
### Requirement: Recover from live Git facts
Create SHALL recover from an interrupted worktree operation by inspecting current Git worktrees, reusing matching targets, and creating only missing targets without a persistent action journal.

#### Scenario: Retry after interruption
- **WHEN** create partially creates worktrees and is run again after the external fault is fixed
- **THEN** matching worktrees are reused and missing worktrees are created
