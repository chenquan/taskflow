## Why

The CLI calculates task plans, status, validation, and session state but does not consistently expose or enforce them. This leaves operators unable to use default output for workflows and allows invalid readiness/configuration states to pass.

## What Changes

- Render result data in default text output.
- Order validation by repository dependencies.
- Include active sessions and dirty worktrees in status/readiness behavior.
- Reject invalid timeouts and OpenSpec change IDs during configuration validation.

## Capabilities

### New Capabilities

- `reporting-validation-readiness`: Make text reporting, dependency-aware validation, session visibility, and finish blocking behavior operationally reliable.

### Modified Capabilities

None.

## Impact

Updates report rendering, application validation/status/finish services, session read APIs, strict configuration validation, and regression tests without changing the public CLI command names or YAML schema.
