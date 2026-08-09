## 1. Validation and Process Adapters

- [x] 1.1 Separate structural task validation from injected Git source-worktree validation while preserving `config validate` diagnostics.
- [x] 1.2 Canonically merge command environments with overlay precedence and Windows key semantics.
- [x] 1.3 Parse NUL-delimited Git porcelain status records for accurate dirty-file counts.
- [x] 1.4 Add OpenSpec semantic-version probing and supported-range validation.

## 2. Start and Doctor Coordination

- [x] 2.1 Reuse the OpenSpec probe in doctor and execute-mode start before state mutation.
- [x] 2.2 Implement deterministic common-dir source-branch locking with correct acquisition, conflict, environment-error, and release behavior.
- [x] 2.3 Gate Git mutation on all source locks and update stable diagnostics and exit-code handling.

## 3. Quality Gates and Regression Coverage

- [x] 3.1 Make snapshot CI wait for test, race, and coverage jobs.
- [x] 3.2 Add unit, application, and end-to-end coverage for validation separation, environment merge, NUL status, version boundaries, and cross-task locks.
- [x] 3.3 Run formatting, full tests, race, vet, coverage, strict OpenSpec validation, Windows compilation, snapshot validation, and diff checks.
