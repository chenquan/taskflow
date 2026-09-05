## MODIFIED Requirements

### Requirement: Exercise the complete CLI task lifecycle
The test suite SHALL execute the reduced user-facing flow through the Cobra command surface and SHALL verify each command's result, configuration, complete source-copy behavior, and relevant Git side effects.

#### Scenario: In-process create succeeds with complete source copies
- **WHEN** a test runs create --dry-run and create --execute against temporary valid repositories containing tracked modifications, untracked files, and ignored files
- **THEN** dry-run reports Worktree and source-copy actions without mutation, execute writes taskflow.yaml and ownership metadata, creates Worktrees, copies the complete source contents except all `.git` entries, and copied tracked modifications appear as unstaged changes

#### Scenario: Dry-run preserves all managed state
- **WHEN** create dry-run is executed for a new or existing task
- **THEN** taskflow.yaml, ownership, lock artifacts, Worktrees, branches, source checkout, and target directories remain unchanged

#### Scenario: Repeated execute is idempotent
- **WHEN** create --execute is run twice for the same task
- **THEN** the second command succeeds, reuses every matching Worktree and complete source copy, and creates no duplicate branch, directory, or copy

#### Scenario: Pending source copy retries safely
- **WHEN** source copying is interrupted after Worktree registration and the fault is removed
- **THEN** the first command reports partial completion and the retry repairs the pending source copy without invoking git worktree add again

### Requirement: Verify executable and rendering boundaries
The test suite SHALL invoke a built taskflow binary in addition to in-process Cobra commands and SHALL verify stable JSON/text output and process exit codes for create, delete, and complete source-copy actions.

#### Scenario: Subprocess reports a complete-copy action
- **WHEN** the built binary runs create dry-run and execute against a temporary repository containing all categories of source files
- **THEN** process exit codes, JSON envelopes, text reports, target contents, and target Git metadata match the complete-copy contract

#### Scenario: Subprocess reports a copy failure
- **WHEN** the built binary encounters an unsupported source entry or filesystem copy failure
- **THEN** it exits with the documented partial or execution code and emits a parseable structured diagnostic without claiming complete source copying
