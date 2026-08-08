## Context

Specflow exposes a Cobra command tree whose commands load and mutate a task workspace through the application service. Existing tests mostly call service methods directly, so they do not verify flag parsing, command output, structured exit errors, or the behavior of the compiled CLI. The tests must remain deterministic across developer machines and CI environments.

## Goals / Non-Goals

**Goals:**

- Cover the lifecycle `init`, `config validate`, `start --dry-run`, `start --execute`, `status`, `validate`, and `finish --dry-run`.
- Verify both in-process Cobra execution and real subprocess exit behavior.
- Use real Git worktrees while controlling OpenSpec behavior with a temporary executable fixture.
- Assert that dry-run is non-mutating and execute mode creates the expected state.

**Non-Goals:**

- Do not change production command semantics or add a test-only production interface.
- Do not require the host to have a particular OpenSpec installation or version.
- Do not test every individual diagnostic already covered by unit tests.

## Decisions

- Keep the E2E tests in the `cmd` package so they exercise `NewRootCommand`, flag wiring, report rendering, and `exitError` directly.
- Create a temporary Git repository with an initial commit and an `openspec` directory, then prepend a temporary directory containing a small `openspec` shell fixture to `PATH`. The fixture implements only `openspec new change <id> --json`, creates the change directory and an unchecked `tasks.md`, and emits valid JSON.
- Use a helper that constructs a fresh Cobra root per command, sets `--tasks-root`, captures stdout/stderr, and returns the command error. Use a separate helper to build and invoke the binary with `os/exec` so process exit codes are observed independently.
- After execute mode creates the change, mark its tasks complete and commit the managed worktree before the success validation/finish assertions. Also retain a dirty/incomplete assertion to prove readiness blockers are surfaced.
- Snapshot the initialized state and source Git facts before dry-run, then compare them afterward. Log calls made to the OpenSpec fixture so a repeated execute can prove that change creation was not invoked twice.
- Run the compiled binary through the same lifecycle phases rather than limiting subprocess coverage to argument failures, while keeping detailed edge-case assertions in the faster in-process suite.
- Install temporary `codex` and `claude` executables beside the OpenSpec fixture. Their invocation log records executable name, working directory, environment overlay, and arguments so `doctor` and `open` remain deterministic and non-interactive.
- Give the OpenSpec and development-tool fixtures fail-once and block/release control files. Tests wait for ready signals with bounded deadlines, assert competing-command behavior, and always release blocked subprocesses during cleanup.
- Organize coverage into complete in-process and binary lifecycles plus focused failure-recovery and conflict/invalid-request suites. Use strongly typed JSON envelopes and state/status payloads for assertions.
- Avoid external Go dependencies; use `testing`, `os/exec`, JSON decoding, and existing repository helpers only.

## Risks / Trade-offs

- [Risk] Shell fixture portability can vary by platform -> use POSIX shell syntax and skip only when the required `git` executable is unavailable.
- [Risk] Building a subprocess from a package test can be slower -> build once per test and remove the binary via `t.TempDir()` cleanup.
- [Risk] Worktree state can leak between commands -> keep all repositories and task roots under `t.TempDir()` and create a new Cobra root for every invocation.
- [Risk] Duplicate lifecycle suites increase runtime -> build the binary once per subprocess test and reuse compact fixture helpers.
- [Risk] Tool launch tests could accidentally invoke installed developer tools -> prepend the fixture directory to `PATH` and assert its invocation log.
- [Risk] Blocking fixtures could hang CI -> bound both shell-side waits and Go-side polling, register release cleanup before process start, and fail with captured output.
