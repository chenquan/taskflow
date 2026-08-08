## Why

Specflow currently embeds OpenSpec change lifecycle management into every managed task, requiring a separate executable and repository initialization even when the user only needs Git worktree coordination. Removing that runtime dependency makes the CLI usable for plain Git repositories while preserving the repository's existing OpenSpec planning materials and workflow resources.

## What Changes

- **BREAKING** Remove OpenSpec runtime integration from Specflow: no probing, compatibility checks, change creation, status inspection, strict validation, or task-completion gating.
- **BREAKING** Remove OpenSpec-specific fields from newly written task configuration and runtime state/output models, including per-repository change identity and OpenSpec summaries.
- Accept the legacy `execution.create_openspec_change` YAML field when reading existing task configurations, but ignore it and omit it from newly generated or displayed configuration.
- Preserve non-code OpenSpec artifacts, skills, and planning documentation unchanged; remove OpenSpec installation and validation from CI.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `task-configuration-validation`: Remove the active OpenSpec creation setting and change-derived repository identity while retaining legacy input compatibility.
- `task-workspace-initialization`: Initialize a task workspace without producing OpenSpec runtime configuration.
- `environment-preflight`: Diagnose Git, development tools, repositories, worktree safety, dependencies, and configured checks without OpenSpec requirements.
- `worktree-start`: Plan and create safe, idempotent worktrees without OpenSpec actions or preflight.
- `aggregate-status-validation`: Report and validate Git/worktree and configured-check facts without OpenSpec status or completion gates.
- `cli-output-contract`: Remove the OpenSpec tool-compatibility condition from CLI behavior.
- `readiness-and-initialization-integrity`: Determine readiness without inspecting OpenSpec task lists.

## Impact

Affected Go packages include `internal/app`, `internal/config`, `internal/domain`, `internal/git`, `internal/plan`, `cmd`, test fixtures, and the removed `internal/openspec` package, plus the CI workflow. The task configuration and JSON result shapes change; existing YAML files with the deprecated creation flag remain loadable.
