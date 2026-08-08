## Why

Teams coordinating one feature across local Git repositories need a safe, repeatable way to record repository ownership and prepare a shared development workspace. The repository currently contains only the generated Cobra skeleton, so it cannot create or validate that workspace without ad-hoc scripts and unsafe assumptions.

## What Changes

- Introduce a Go `specflow` CLI foundation with consistent human-readable and JSON output.
- Add strict YAML configuration and domain validation for a requirement workspace and its local Git repositories.
- Add safe initialization that creates requirement metadata without creating Git branches, worktrees, or OpenSpec changes.
- Add read-only configuration inspection and environment/repository preflight diagnostics.
- Add atomic state writes and an exclusive task lock for mutating initialization operations.

## Capabilities

### New Capabilities

- `task-workspace-initialization`: Create an idempotent requirement control directory and associate explicitly declared local Git repositories without changing their Git state.
- `task-configuration-validation`: Load, display, and strictly validate the versioned YAML task configuration, including paths and repository dependency relationships.
- `environment-preflight`: Report deterministic local tool, repository, base-reference, OpenSpec, worktree-target, and configured-check readiness diagnostics.
- `cli-output-contract`: Provide a stable command result envelope and documented exit-code behavior for text and JSON CLI output.

### Modified Capabilities

None.

## Impact

This adds Cobra command bindings and focused Go packages under `internal/` for domain models, configuration, safe filesystem operations, command execution, Git/OpenSpec inspection, locking, application services, and reporting. It upgrades the module's Go baseline and adds YAML and file-lock dependencies. Worktree creation, development-tool launching, validation execution, archive, cleanup, push, and PR behavior remain out of scope for this change.
