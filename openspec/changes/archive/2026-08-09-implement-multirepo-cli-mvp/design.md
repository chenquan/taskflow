## Context

`specflow` is a new Go module with a generated Cobra root command but no domain behavior. The accompanying implementation plan defines a multi-repository coordinator whose requirement directory is intentionally separate from every business repository. This first change implements the plan's phases 0 and 1 only: safe workspace initialization and diagnostics, before any worktree or OpenSpec change creation.

The CLI must work on macOS, Linux, and Windows, must treat YAML as untrusted input, and must not turn a validation or diagnostic command into a Git mutation.

## Goals / Non-Goals

**Goals:**

- Provide a testable Go package boundary between Cobra bindings and application behavior.
- Define a strict, versioned YAML task configuration and stable JSON result envelope.
- Create a task directory safely and idempotently from explicit `--repo name=path` inputs.
- Discover enough Git and OpenSpec state for inventory and doctor reports without fetching or changing repositories.
- Enforce path containment, dependency-graph validity, atomic file writes, and a task-scoped exclusive lock for `init`.

**Non-Goals:**

- Creating branches, worktrees, OpenSpec changes, worksets, commits, pushes, PRs, archives, or cleanup.
- Interactive repository scanning and development-tool process launching.
- Persisting credentials, arbitrary environment variables, or shell command strings.
- Cross-repository transactions or automatic recovery that deletes user files.

## Decisions

### Layered packages with dependency injection

Commands only parse flags and render a `report.Result`; application services receive interfaces for filesystem, process execution, Git, OpenSpec, and locking. This keeps policy testable without a Cobra process and permits fake adapters in unit tests.

Keeping all behavior in Cobra `RunE` was rejected because it would couple safety checks and process execution to terminal presentation. A generic plugin architecture was rejected for the first slice because no external adapter is yet needed.

### Strict configuration is the source of intent

`specflow.yaml` carries task metadata and repository declarations; `.specflow/inventory.json` and `.specflow/state.json` are derived snapshots. YAML uses `yaml.v3` with known-fields enforcement, supported-version checking, canonical absolute paths, unique repository names, a valid primary repository, contained worktree paths, and acyclic dependencies.

Permissive YAML decoding was rejected because misspelled safety fields could otherwise be silently ignored. State is not treated as authoritative because repositories and files may change outside the CLI.

### Explicit, no-shell execution

All external probes use `exec.CommandContext` with executable and argument arrays. The initial Git adapter uses read-only commands (`rev-parse`, `remote -v`, `symbolic-ref`, `status --porcelain`, `show-ref`, `worktree list --porcelain`); the OpenSpec adapter only probes availability and repository initialization. Configured checks are inspected for existence only in this slice.

Shell command strings are rejected because they make quoting and injection behavior platform-dependent. `init` never invokes `git fetch`, `git worktree`, or `openspec new`.

### Safe initialization and persistence

`init` requires an explicit tasks root and at least one named repository in non-interactive operation. It acquires `.specflow/lock`, creates only the requirement directory and metadata files, and writes files through a same-directory temporary file followed by rename. Existing equivalent configuration succeeds; differing configuration fails without overwriting it. Existing non-managed content is never overwritten.

The lock is file-based so concurrent CLI processes coordinate on the same requirement directory. It fails quickly with a conflict result rather than waiting indefinitely.

### One result model for text and JSON

Every command returns `schemaVersion`, `command`, `ok`, optional `taskID`, `data`, `warnings`, and structured errors. Cobra maps typed errors to documented exit codes. JSON rendering uses only the result model and never emits ANSI sequences, ensuring machine consumers see the same facts as terminal users.

## Risks / Trade-offs

- [Git or OpenSpec CLI output differs across installed versions] → adapters parse only required fields and return a compatibility diagnostic with captured command context.
- [Canonicalization follows an unavailable or permission-restricted path] → require existing source repositories, canonicalize the existing parent for managed paths, and report a configuration error rather than guessing.
- [A process dies while holding a lock] → OS-backed lock release handles normal process termination; lock metadata is informational and no stale file is deleted automatically.
- [A valid repository is dirty] → doctor reports a warning, not a blocker, because initialization is read-only.
- [A generated metadata file is interrupted] → atomic write leaves either the previous full file or the new full file, never a partial document.

## Migration Plan

1. Replace the generated Cobra placeholder with the root command and phase 0–1 subcommands.
2. Add domain/config/reporting packages and tests before connecting adapters.
3. Add initialization, config, and doctor application services with fake-adapter tests.
4. Add real read-only Git/OpenSpec adapters and integration tests against temporary repositories.
5. Run formatting, unit tests, vet, and strict OpenSpec validation. No user workspace migration is required because this is the first released configuration version.

## Open Questions

- `--scan` is specified in the long-term UX but has no fixed interaction design; it is intentionally deferred rather than partially implemented.
- The supported OpenSpec CLI version range will be finalized when phase 2 needs change creation; phase 1 detects availability and initialization only.
