## 1. Replace the persisted domain and configuration contract

- [x] 1.1 Remove state, validation, inventory, dependency, check, and fetch fields from the domain model; retain only task identity and ordered repository worktree configuration.
- [x] 1.2 Update strict YAML loading, normalization, defaults, target containment, duplicate-target validation, and current config version for the reduced taskflow.yaml contract.
- [x] 1.3 Replace dependency/fetch action planning with declaration-order create/reuse action planning and remove obsolete state/report persistence helpers.

## 2. Implement state-free worktree reconciliation

- [x] 2.1 Add create options and configuration resolution for new tasks, existing tasks, and append-only `--repo` declarations.
- [x] 2.2 Implement full read-only create preflight for source identity, local base, source-branch occupancy, target registration, target identity, and conflict diagnostics.
- [x] 2.3 Implement dry-run output with zero task/configuration/lock/Git mutation and execute-mode task/source lock ordering.
- [x] 2.4 Persist taskflow.yaml atomically after successful preflight, create only missing worktrees, reuse matching worktrees, and preserve retryability after partial Git failure.
- [x] 2.5 Remove state-dependent Open gating and make open use live worktree identity checks while retaining safe Codex/Claude launch construction and child-process behavior.

## 3. Reduce the public CLI

- [x] 3.1 Replace init/start with create flags and defaults, preserving JSON/text rendering and stable exit codes.
- [x] 3.2 Remove status, validate, repo add, and other lifecycle command registrations while retaining version and the two operational commands.
- [x] 3.3 Update command-level diagnostics, dry-run/execute semantics, tool passthrough, and nested-worktree argument rejection for create/open.

## 4. Update guidance and specifications

- [x] 4.1 Rewrite README command examples, taskflow.yaml schema, task directory layout, recovery behavior, and breaking boundary for create/open and no state.json.
- [x] 4.2 Rewrite the bundled Taskflow skill to use create dry-run/execute and live open readiness without state, validation, or lifecycle instructions.
- [x] 4.3 Run strict OpenSpec validation and reconcile delta-spec details with the implemented contract before verification.

## 5. Replace and expand verification

- [x] 5.1 Rewrite domain/config/app/plan/root tests for the reduced model and remove assertions for state, validation, dependencies, checks, fetch, and retired commands.
- [x] 5.2 Add unit coverage for state-free create planning, append semantics, target identity, duplicate targets, preflight-before-mutation, and open readiness.
- [x] 5.3 Replace lifecycle E2E coverage with create dry-run/execute/retry/idempotence/append/conflict and open Codex/Claude launch-boundary scenarios.
- [x] 5.4 Run gofmt, OpenSpec validation, unit tests, Git-backed E2E tests, vet, race checks, and a final legacy-artifact search.
