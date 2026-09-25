## Why

Taskflow currently defaults new repositories to `HEAD` and a reusable `feature/<task-id>` branch. This can start work from a stale local checkout. New workspaces should follow each repository's configured remote default branch while keeping the existing simple task branch convention.

## What Changes

- Default `init` and `repo add` bases become each source repository's `origin/<default-branch>` reference resolved from `origin/HEAD`.
- Default branches remain `feature/<task-id>` and are generated consistently for every repository in a task.
- `repo add` uses the same task branch rather than generating a different branch for the appended repository.
- Explicit `base` and `branch` values remain authoritative; existing task configuration is not rewritten.
- Missing or unusable `origin/HEAD` is reported before task metadata is written.

## Capabilities

### New Capabilities

- `remote-default-based-workspaces`: Resolve repository defaults from the remote default branch and generate task-scoped branches.

### Modified Capabilities

- `task-workspace-initialization`: Change the defaults used when initializing repositories.
- `repository-append`: Change the defaults used when appending repositories.

## Impact

Affected areas include the Git inspection adapter, initialization and repository-append application services, configuration/defaulting tests, user documentation, bundled Taskflow skill guidance, and OpenSpec capability requirements. No new external dependency or persisted schema version is required.
