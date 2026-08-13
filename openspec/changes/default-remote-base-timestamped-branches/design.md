## Context

`init` currently inspects each source repository but constructs repositories with `base: HEAD` and a reusable `feature/<task-id>` branch. `repo add` duplicates those defaults. The Git adapter already observes `origin/HEAD`, so the change can stay within the existing application and Git layers without changing the persisted schema.

## Goals / Non-Goals

**Goals:**

- Resolve each new repository's default base from its `origin/HEAD` symbolic reference.
- Generate the same `feature/<task-id>` default branch for every repository in a task.
- Preserve explicit configuration and existing task files.
- Reject missing remote-default information before metadata mutation.

**Non-Goals:**

- Rewriting or migrating existing task configurations.
- Fetching during `init` or `repo add`.
- Changing worktree creation, branch conflict handling, or the explicit `start --execute` approval gate.

## Decisions

- **Resolve the default base per source repository.** Use `git symbolic-ref --short refs/remotes/origin/HEAD`, normalize the result to `origin/<branch>`, and require that reference to resolve locally. This supports repositories whose default branch is not `main`; falling back to `HEAD` would undermine the requested remote baseline.
- **Keep branch generation at task scope.** `init` applies the same `feature/<task-id>` branch to all repositories. `repo add` uses the same derived task branch, preserving the existing multi-repository convention without adding a timestamp component.
- **Keep defaulting in the application service.** `config.Validate` remains structural and side-effect free. `init` and `repo add` perform Git-dependent default resolution before calling validation and writing metadata.
- **Keep the existing branch format.** The branch format is `feature/<lowercase-task-id>`; the existing task ID normalization is retained.

## Risks / Trade-offs

- [A repository has no `origin/HEAD`] → fail initialization or append before writing and report the repository name plus the required remote-default setup.
- [The remote default ref is stale] → `start --execute` retains the existing optional fetch behavior; initialization itself remains metadata-only.
- [A task created before this change has a custom branch] → preserve it; only new defaults and append operations requiring a task-derived timestamp are affected.

## Migration Plan

No data migration is performed. Deploy the new binary; existing `taskflow.yaml` values remain authoritative. Rollback is a code-version rollback because no schema or state migration is introduced.

## Open Questions

None.
