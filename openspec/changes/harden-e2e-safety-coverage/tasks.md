## 1. E2E fixture infrastructure

- [x] 1.1 Restore portable real-Git, fixture-executable, state-snapshot, Cobra, JSON, and compiled-binary helpers without OpenSpec support.
- [x] 1.2 Upgrade the Go directive to 1.25 and keep all test fixtures compatible with CI platforms.

## 2. Lifecycle and safety coverage

- [x] 2.1 Cover multi-repository dependency ordering, Unicode paths, dry-run immutability, idempotent start, scoped validation, and non-mutating finish.
- [x] 2.2 Cover preflight target/branch conflicts, source locks, invalid requests, JSON exit codes, and compiled-binary lifecycle behavior.
- [x] 2.3 Cover check failures/timeouts, stale validation, dirty/head changes, and development-tool/session failure cleanup.

## 3. Validation

- [x] 3.1 Run formatting, unit and E2E tests, race tests, vet, strict OpenSpec validation, and diff checks.
- [x] 3.2 Run the CI-equivalent coverage command and satisfy the 80 percent production coverage threshold.
