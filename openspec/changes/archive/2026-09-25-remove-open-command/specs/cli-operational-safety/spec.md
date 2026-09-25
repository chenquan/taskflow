## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: Run interactive tools with streams and environment
**Reason**: The CLI no longer launches child tools; runner streaming and explicit environment overlays existed only for the tool launch path.
**Migration**: `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1` is set as a prefix on the skill-composed `claude` command line that the user runs.
