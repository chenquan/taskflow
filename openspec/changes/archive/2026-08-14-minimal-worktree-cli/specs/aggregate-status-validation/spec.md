## REMOVED Requirements

### Requirement: Aggregate task status
**Reason**: The reduced product has no status command or persisted lifecycle state; create and open report current action and readiness facts directly.
**Migration**: Inspect create/open output or use Git commands in the configured worktrees.

### Requirement: Validate repositories and configured checks
**Reason**: Configured checks and the validate command are outside the worktree creation/opening boundary.
**Migration**: Run repository-specific tests directly in each worktree.

### Requirement: Report stale validation results
**Reason**: Validation reports and configuration digests are removed with stateful validation.
**Migration**: Re-run repository checks directly when needed; no historical report is maintained by Taskflow.

## ADDED Requirements

### Requirement: Do not provide lifecycle reporting or validation
Taskflow MUST NOT provide status or validate commands, persist validation reports, or infer task readiness from historical state. Current create/open action facts MUST be reported directly by those commands.

#### Scenario: Removed reporting command
- **WHEN** a user invokes status or validate
- **THEN** the CLI rejects the retired command and leaves the task workspace unchanged
