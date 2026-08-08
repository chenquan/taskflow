## 1. Worktree and OpenSpec adapters

- [x] 1.1 Add stable Git worktree and OpenSpec models plus porcelain/change-directory inspection methods.
- [x] 1.2 Implement topological repository ordering and deterministic action-plan generation.
- [x] 1.3 Implement argument-array worktree creation and OpenSpec change creation with mismatch checks.

## 2. Resumable start application

- [x] 2.1 Implement dry-run and execute start modes under the task lock.
- [x] 2.2 Persist per-repository action outcomes atomically and resume from actual state after failures.
- [x] 2.3 Bind Cobra `start <task-id> --dry-run|--execute` with text/JSON reports.

## 3. Verification

- [x] 3.1 Add integration tests for three repositories, idempotent reruns, dry-run non-mutation, and mismatch rejection.
- [x] 3.2 Run formatting, tests, vet, diff checks, and strict OpenSpec validation.
