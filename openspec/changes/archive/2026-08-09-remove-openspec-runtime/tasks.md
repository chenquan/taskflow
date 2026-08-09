## 1. Runtime model and compatibility

- [x] 1.1 Remove OpenSpec and change fields from active domain, state, inventory, status, validation, and plan models.
- [x] 1.2 Make configuration loading accept and ignore only the legacy `execution.create_openspec_change` field while retaining strict unknown-field validation.
- [x] 1.3 Update initialization and normalized configuration output to omit retired OpenSpec fields.

## 2. CLI lifecycle behavior

- [x] 2.1 Delete the OpenSpec client package and remove OpenSpec probes and source-directory checks from Git and application services.
- [x] 2.2 Remove OpenSpec actions and preflight from `start`, retaining existing safe, idempotent worktree behavior.
- [x] 2.3 Remove OpenSpec data and readiness gates from doctor, status, validation, finish, reports, and CLI help text.

## 3. Tests and verification

- [x] 3.1 Remove OpenSpec-specific unit, integration, and E2E fixtures; update remaining test expectations to the reduced public data model.
- [x] 3.2 Add coverage for legacy YAML compatibility, OpenSpec-free lifecycle execution, and the absence of OpenSpec output and command invocation.
- [x] 3.3 Remove OpenSpec setup and validation from CI while retaining Go quality gates; run Go tests, race tests, `go vet`, and `git diff --check` without modifying retained OpenSpec skills, artifacts, or planning documentation.
