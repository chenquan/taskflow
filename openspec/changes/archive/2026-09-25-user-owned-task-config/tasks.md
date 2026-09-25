## 1. Restrict create configuration input

- [x] 1.1 Change create resolution so `--repo` is accepted only when taskflow.yaml is absent and return `CONFIG_EDIT_REQUIRED` for existing tasks before any mutation.
- [x] 1.2 Remove append-only configuration persistence and update Cobra help, diagnostic data, and create result semantics for bootstrap-only repository arguments.

## 2. Update user and agent contracts

- [x] 2.1 Rewrite README examples and configuration guidance to use direct taskflow.yaml edits for existing tasks.
- [x] 2.2 Rewrite the bundled Taskflow skill to distinguish bootstrap from direct-config reconciliation and preserve dry-run/execute approval guidance.
- [x] 2.3 Update active OpenSpec main specifications and remove the active repository-append capability while leaving archived history unchanged.

## 3. Replace verification coverage

- [x] 3.1 Replace app and CLI append tests with existing-task `CONFIG_EDIT_REQUIRED` tests that prove no configuration or Git mutation.
- [x] 3.2 Add direct taskflow.yaml add/remove/edit reconciliation tests, including preservation of unlisted worktrees and conflict safety.
- [x] 3.3 Update Git-backed E2E and skill-content tests for bootstrap-only `--repo`, direct configuration edits, JSON diagnostics, and unchanged open behavior.

## 4. Verify and finalize

- [x] 4.1 Run formatting, strict OpenSpec validation, unit tests, E2E tests, vet, race, build, and legacy-artifact searches.
- [x] 4.2 Review the final diff for stale append wording and report the changed contract and verification results.
