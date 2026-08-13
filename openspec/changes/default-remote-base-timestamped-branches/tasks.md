## 1. Git and default-value resolution

- [x] 1.1 Add Git-client helpers and diagnostics for resolving a source repository's usable `origin/<default-branch>` from `origin/HEAD`.
- [x] 1.2 Add task-level default branch generation using `feature/<task-id>`.
- [x] 1.3 Update `init` to resolve per-repository remote bases, generate one shared default branch, and fail before metadata writes when resolution fails.
- [x] 1.4 Update `repo add` to resolve the appended repository base and use the task default branch.

## 2. Verification and contracts

- [x] 2.1 Add unit and application tests for non-main remote defaults, missing refs, shared init timestamps, repo-add inheritance, and explicit-value preservation.
- [x] 2.2 Update README and bundled Taskflow skill documentation for the new defaults and safe start workflow.
- [x] 2.3 Run Go tests, vet, diff checks, and strict OpenSpec validation; fix any regressions.
