## 1. OpenSpec Artifacts

- [x] 1.1 Define the E2E lifecycle and executable-boundary requirements.
- [x] 1.2 Document the deterministic Git/OpenSpec fixture design.

## 2. In-Process Cobra Coverage

- [x] 2.1 Add temporary Git/OpenSpec fixture helpers and a fresh-root Cobra command runner.
- [x] 2.2 Add the successful lifecycle test with dry-run, execute, status, validation, and finish assertions.
- [x] 2.3 Add incomplete-task and dirty-worktree readiness failure assertions.

## 3. Binary Boundary Coverage

- [x] 3.1 Add a test helper that builds the CLI binary and executes it with captured output and environment.
- [x] 3.2 Verify subprocess JSON diagnostics, configuration exit codes, and successful text data rendering.

## 4. Verification

- [x] 4.1 Run Go tests, race tests, OpenSpec strict validation, and `git diff --check`.

## 5. Review Hardening

- [x] 5.1 Assert dry-run preserves the state file, source Git status, feature refs, and OpenSpec state.
- [x] 5.2 Assert repeated execute reuses one worktree and invokes OpenSpec creation only once.
- [x] 5.3 Extend the compiled-binary test through execute, status, validation, and finish.
- [x] 5.4 Parse JSON envelopes structurally and rerun all verification commands.
- [x] 5.5 Cover `doctor` and non-interactive `open` launches for Codex and Claude fixtures.

## 6. Complete Lifecycle Coverage

- [x] 6.1 Extend fixtures with check logging, fail-once behavior, and bounded block/release controls.
- [x] 6.2 Cover version, idempotent init, config show/validate, repository-scoped doctor, configured checks, and readiness-only finish.
- [x] 6.3 Add start failure/resume and mismatched-target preservation scenarios.
- [x] 6.4 Add deterministic task-lock and active-session conflict scenarios with cleanup.
- [x] 6.5 Add invalid flag, tool, task ID, and configuration scenarios with structured exit-code assertions.
- [x] 6.6 Repeat E2E tests and run the full test, race, vet, OpenSpec, and diff verification suite.
