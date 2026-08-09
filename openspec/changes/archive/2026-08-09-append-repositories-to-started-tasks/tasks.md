## 1. Domain and status staleness

- [x] 1.1 Add `ValidationStale bool` (json `validationStale`) to `domain.StatusData`
- [x] 1.2 Update `app.Status` to compute the current config digest, set `ValidationStale` when the persisted report digest differs, omit stale `lastValidation` and per-repository `lastValidationOK`, and leave `validation.json` on disk
- [x] 1.3 Verify the next `validate` regenerates a digest-matching report and clears staleness

## 2. RepoAdd application service

- [x] 2.1 Add `RepoAddOptions` and `Service.RepoAdd` in `internal/app`, returning `report.New("repo add", ...)`
- [x] 2.2 Parse `name=path` and validate path existence, Git repository, name uniqueness, and `--depends-on` references (existing repos only, not the appended repo)
- [x] 2.3 Build the appended repository with `init` defaults and run `config.Validate` on the merged task
- [x] 2.4 Load state, gate by `initialized`/`started`/`failed` phase, and reject other phases before writing
- [x] 2.5 Compute the merged digest, preserve existing repository outcomes, and add pending fetch (or skipped when `execution.fetch` is false) and pending worktree actions for the appended repository
- [x] 2.6 Implement the snapshot-and-restore writer that updates `taskflow.yaml`, appends inventory facts, and writes `state.json` atomically, rolling back all three on any write failure
- [x] 2.7 Support `--dry-run` to return the resolved repository and its pending start actions without acquiring the lock or writing

## 3. CLI command

- [x] 3.1 Add a `repo` command group with an `add <task-id>` subcommand exposing `--repo` (required), `--depends-on` (repeatable), and `--dry-run`
- [x] 3.2 Wire `svc.Load` failure to `INVALID_CONFIGURATION` and render the `repo add` result and exit code

## 4. Unit tests

- [x] 4.1 Append to `initialized`, `started`, and `failed` tasks and assert pending state plus preserved outcomes
- [x] 4.2 Assert errors for duplicate name, non-Git path, unknown/self dependency, unsupported phase, and state digest conflict on the next start
- [x] 4.3 Assert `status` reports `validationStale` after append and recovers after `validate`
- [x] 4.4 Assert a mid-sequence write failure rolls back `taskflow.yaml`, `inventory.json`, and `state.json`

## 5. E2E tests

- [x] 5.1 Create and start a single-repository task, then append a second repository
- [x] 5.2 Assert `start --dry-run` reports only the appended repository's actions and mutates nothing
- [x] 5.3 Assert `start --execute` reuses the existing worktree and creates only the appended worktree, and repeat execution is idempotent
- [x] 5.4 Assert a task-lock conflict leaves configuration, state, and existing worktrees unchanged

## 6. Documentation

- [x] 6.1 Add the append workflow, phase constraints, and dry-run/execute sequence to the README
- [x] 6.2 Add `repo add` guidance to the Taskflow skill
