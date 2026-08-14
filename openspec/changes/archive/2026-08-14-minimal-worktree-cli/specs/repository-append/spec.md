## REMOVED Requirements

### Requirement: Append repositories to an existing task
**Reason**: Independent `repo add` is removed; repository growth is part of create reconciliation.
**Migration**: Invoke `create <task-id> --repo <name>=<absolute-path>`; existing declarations remain append-only.

### Requirement: Gate append by task phase and task lock
**Reason**: Task phases and append-specific lifecycle gates are removed.
**Migration**: Create always uses the task lock and live preflight for the complete desired configuration.

### Requirement: Migrate configuration digest atomically
**Reason**: State, config digest, inventory, and pending action migration are removed.
**Migration**: Create atomically writes the complete taskflow.yaml after preflight; retry uses live Git state.

### Requirement: Preview an append with dry-run
**Reason**: Append is now a create input and uses create dry-run.
**Migration**: Use `create --dry-run --repo <name>=<absolute-path>`.

## ADDED Requirements

### Requirement: Fold repository growth into create
Create SHALL accept new unique repository declarations for an existing task, preserve existing declaration order, and reconcile the newly declared worktree without an independent repository command.

#### Scenario: Append through create
- **WHEN** an existing task receives a new repository with `create --repo`
- **THEN** create previews or creates only the new worktree and preserves existing repositories
