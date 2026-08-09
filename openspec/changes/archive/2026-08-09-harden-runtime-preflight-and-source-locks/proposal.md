## Why

Specflow currently mixes configuration validation with unbounded direct Git execution, can pass duplicate environment variables to child tools, and detects some OpenSpec incompatibilities only after execution state is written. Separate tasks can also race while creating the same source-repository branch.

## What Changes

- Separate pure task-configuration validation from injected Git environment validation while preserving `config validate` behavior.
- Canonically merge child-process environment overlays and parse Git status with NUL-delimited porcelain output.
- Require an OpenSpec version in the supported `>=1.4.1, <2.0.0` range before execute-mode mutations.
- Add deterministic local source-and-branch locks shared across task roots.
- Make CI snapshot creation wait for test, race, and coverage gates.

## Capabilities

### New Capabilities

- `cross-task-source-coordination`: Serialize competing local tasks that target the same Git source branch.

### Modified Capabilities

- `task-configuration-validation`: Separate structural validation from environment inspection without weakening `config validate`.
- `environment-preflight`: Report supported OpenSpec compatibility instead of merely non-empty version output.
- `worktree-start`: Probe compatible OpenSpec and acquire source-branch locks before any execute-mode mutation.
- `development-tool-sessions`: Make launch environment overlays deterministic across supported operating systems.
- `cli-output-contract`: Report source-branch conflicts and OpenSpec incompatibility with stable exit codes.

## Impact

Affected areas are configuration loading, the Git/OpenSpec/process adapters, start and doctor orchestration, lock storage beneath local Git metadata, lifecycle tests, and the GitHub Actions workflow. YAML schema, existing command flags, and state/report schemas remain unchanged.
