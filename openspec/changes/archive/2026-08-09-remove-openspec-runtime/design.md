## Context

The current Go CLI couples worktree lifecycle operations to OpenSpec through `internal/openspec`, task configuration, repository-derived change IDs, persisted action state, and status/validation/readiness output. The requested scope removes this executable/runtime integration and the CI steps that install or validate OpenSpec. Repository-local planning materials, agent skills, and implementation documentation remain untouched even where they describe the prior behavior.

## Goals / Non-Goals

**Goals:**

- Make all CLI lifecycle commands work without the OpenSpec executable or an `openspec/` directory in managed source repositories.
- Remove OpenSpec-specific runtime fields, actions, diagnostics, output, tests, and dependencies from Go code.
- Remove OpenSpec setup and validation from CI while retaining the Go quality gates.
- Keep pre-existing task YAML loadable when it contains the deprecated creation flag.

**Non-Goals:**

- Delete or rewrite repository OpenSpec artifacts, skills, or planning documents.
- Migrate already-persisted state files, preserve their removed output fields, or introduce another planning system.
- Change Git safety, dependency ordering, configured checks, worktree locking, or development-tool behavior.

## Decisions

### Treat OpenSpec as a legacy input only

The configuration loader will recognize `execution.create_openspec_change` only to accept existing YAML, discard it from the decoded model, and continue rejecting every other unknown field. New configuration serialization has no equivalent field. This avoids breaking existing task workspaces without retaining OpenSpec in the public active model.

Alternative considered: reject the field immediately. This is simpler but unnecessarily invalidates existing task workspaces.

### Remove the complete runtime data path

Remove repository change IDs, OpenSpec action outcomes, OpenSpec inventory/status/validation summaries, adapter calls, and OpenSpec diagnostics together. `start` plans only directory/fetch/worktree actions; `doctor`, `status`, `validate`, and `finish` derive outcomes only from configuration, Git, dependency state, and configured checks.

Alternative considered: retain empty or always-successful OpenSpec output fields. That would preserve a misleading public API and leave dead code paths.

### Preserve non-code workflow materials verbatim

No files in `openspec/`, `.codex/`, `.claude/`, `.agents/`, or the implementation-plan document are edited by this change. In `.github/workflows/ci.yml`, remove only the Node setup, OpenSpec installation, and OpenSpec validation steps; retain all Go test, race, coverage, and snapshot jobs. The retained OpenSpec references remain project history and contributor workflow material, not a statement of the resulting CLI contract.

Alternative considered: update all references for consistency. The user explicitly constrained this change to code removal.

## Risks / Trade-offs

- [Retained documents describe the previous runtime contract] → Limit documentation changes to the user-approved CI cleanup and use the resulting code tests as behavioral authority.
- [Old JSON consumers expect `openSpec` or `change`] → Mark the output-shape removal as breaking and test the new stable shape.
- [Strict decoding accidentally rejects legacy YAML] → Add a dedicated compatibility decoder test while retaining strict rejection for unrelated unknown fields.
- [Persisted old state contains obsolete fields] → Go's JSON decoder ignores those fields; no state migration is required.

## Migration Plan

1. Release the code change with legacy YAML compatibility enabled.
2. Existing task configurations continue to run; newly initialized task workspaces omit the retired field.
3. Consumers of JSON result data remove reads of `change` and `openSpec`; no rollback data conversion is needed because the prior code remains compatible with its persisted files.

## Open Questions

None.
