## Why

Readiness reporting currently treats the presence of an OpenSpec change directory as complete work, configuration accepts bare Git repositories as sources, and invalid initialization can leave an empty task directory behind. These gaps make automation report work as ready when it is not and leave avoidable partial workspace state.

## What Changes

- Inspect OpenSpec task checkboxes during validation and block finish reports when work remains.
- Require repository sources to be non-bare Git worktrees rather than only accepting a successful Git command exit status.
- Validate the complete initialization configuration before creating a task workspace directory.

## Capabilities

### New Capabilities

- `readiness-and-initialization-integrity`: Reliable OpenSpec readiness validation, Git worktree source validation, and side-effect-free rejection of invalid initialization input.

### Modified Capabilities

<!-- No existing main specs are present in this repository. -->

## Impact

- `internal/app` validation, finish, and initialization flow.
- `internal/config` Git source inspection.
- Application and configuration tests, plus the new OpenSpec requirement.
