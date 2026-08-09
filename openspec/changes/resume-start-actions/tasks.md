## 1. State model and loading

- [x] 1.1 Add an additive configuration digest field to persisted task state and helpers to load, validate, and classify missing, malformed, or incompatible state.
- [x] 1.2 Add unit tests for legacy empty-digest state, digest mismatch, malformed state, and atomic state persistence.

## 2. Resumable start execution

- [x] 2.1 Preserve compatible prior state after preflight instead of rebuilding all actions as pending.
- [x] 2.2 Reconcile directory, fetch, and worktree completion from current filesystem/Git facts before each action.
- [x] 2.3 Skip completed actions safely, rerun only incomplete or no-longer-valid actions, and persist outcomes after each transition.
- [x] 2.4 Return structured state-conflict or state-incompatible diagnostics before mutation and preserve the original state file.

## 3. Regression coverage

- [x] 3.1 Add service-level tests for resumed state transitions and action skipping.
- [x] 3.2 Extend CLI E2E coverage for partial failure followed by retry, completed fetch reuse, missing worktree recreation, digest mismatch, and corrupt state.
- [x] 3.3 Verify existing lock, preflight, idempotency, JSON/text output, and cross-platform quality checks remain green.

## 4. Validation and handoff

- [ ] 4.1 Run the full Go test, race, vet, coverage, OpenSpec strict validation, and diff checks.
- [x] 4.2 Review the final diff for scope and report remaining limitations or follow-up work.
