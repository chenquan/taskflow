## 1. Tool adapters and leases

- [x] 1.1 Implement Codex/Claude launch-spec adapters and reject dangerous flags.
- [x] 1.2 Implement task session lease acquisition, PID liveness, and release.
- [x] 1.3 Bind Cobra `open` with inherited stdio and child exit-code handling.

## 2. Aggregate operations

- [x] 2.1 Implement Git/OpenSpec status aggregation.
- [x] 2.2 Implement dependency-aware configured-check validation with timeout reporting.
- [x] 2.3 Implement non-mutating `finish --dry-run` readiness report.
- [x] 2.4 Bind `status`, `validate`, and `finish` with text/JSON output.

## 3. Verification

- [x] 3.1 Add adapter, lease, status, validation, and command tests.
- [x] 3.2 Run formatting, tests, vet, diff checks, and strict OpenSpec validation.
