## 1. Foundation and output contract

- [x] 1.1 Upgrade the Go module baseline and add strict YAML and file-lock dependencies.
- [x] 1.2 Define domain models, typed errors, and stable command result/exit-code rendering with unit tests.
- [x] 1.3 Replace the generated Cobra placeholder with root, version, and shared output/flag infrastructure.

## 2. Strict configuration and safe persistence

- [x] 2.1 Implement canonical path helpers and atomic file writing with containment tests.
- [x] 2.2 Implement strict YAML loading, normalization, validation, dependency-cycle detection, and YAML serialization with unit tests.
- [x] 2.3 Implement task-scoped non-blocking file locks and state/inventory persistence with tests.

## 3. Read-only adapters and initialization

- [x] 3.1 Implement a no-shell command runner and read-only Git/OpenSpec inspection adapters with tests.
- [x] 3.2 Implement idempotent explicit-repository task initialization without Git mutations and cover success/conflict cases.
- [x] 3.3 Bind `init` and `config show` / `config validate` commands, including JSON output.

## 4. Preflight diagnostics

- [x] 4.1 Implement doctor diagnostics for tools, Git repositories, base references, OpenSpec setup, worktree targets, and configured checks.
- [x] 4.2 Bind `doctor` including repository filtering and text/JSON presentation.

## 5. Verification and documentation

- [x] 5.1 Add focused CLI/integration tests proving initialization is non-mutating and doctor handles warnings and failures.
- [x] 5.2 Run gofmt, go test, go vet, diff checks, and strict OpenSpec validation; fix all reported issues.
