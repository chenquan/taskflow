## Context

The current E2E suite has one single-repository happy path after removal of OpenSpec fixtures. The CLI still owns cross-repository worktree safety, durable start state, checks, sessions, and rendering contracts that require real subprocess and Git coverage.

## Goals / Non-Goals

**Goals:**

- Restore deterministic, portable E2E coverage without OpenSpec binaries or directories.
- Exercise real Git worktrees and compiled CLI boundaries while using small Go fixture executables for checks and development tools.
- Raise combined production coverage to the existing 80 percent CI threshold.

**Non-Goals:**

- Change production lifecycle behavior, add network dependencies, or restore OpenSpec support.
- Depend on shell scripts, POSIX-only paths, or a host-installed Codex or Claude binary.

## Decisions

### Reuse one Go fixture binary for tools and checks

Build `internal/testfixture` into a temporary executable directory and invoke copies under the configured tool/check names. Environment-controlled logs, blocking, failure, and timeout behavior make subprocess tests deterministic on every CI platform.

### Test safety at the command boundary

Use real temporary Git repositories, Cobra commands, and one compiled-binary smoke path. Assert state snapshots and filesystem absence/preservation for preflight, lock, branch, and target conflicts instead of reaching into application internals.

### Keep OpenSpec absent by construction

Fixture repositories contain only Git state. Every lifecycle assertion checks that generated configuration and JSON output omit OpenSpec fields, so a reintroduced runtime dependency fails the E2E suite.

## Risks / Trade-offs

- [E2E tests are slower] → Share fixture helpers and keep one compiled-binary lifecycle while running most scenarios in-process.
- [Concurrent tests can become flaky] → Use ready/release files with bounded deadlines and cleanup release signals.
- [Coverage varies by platform] → Keep tests platform-neutral and enforce the existing combined threshold in the CI-equivalent command.
