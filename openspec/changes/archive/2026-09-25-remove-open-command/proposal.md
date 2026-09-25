## Why

The `open` command was designed as a fast path to launch Codex or Claude from a prepared workspace, but the bundled skill can guide an agent to generate the native `claude`/`codex` command line for the user to run directly. Wrapping tool launch in Go code adds CLI surface, launch diagnostics, and a preflight path that duplicate what the skill and `create --dry-run` already provide. Removing it keeps the CLI focused on the workspace lifecycle (create/delete) and moves "open a terminal" back to skill guidance.

## What Changes

- **BREAKING** Remove the `taskflow open` CLI command, its `--tool` flag, and the `TOOL_NOT_FOUND`/`TOOL_EXITED` diagnostics.
- **BREAKING** Remove `Service.Open`, `preflightOpen`, and the `internal/devtool` package; Taskflow no longer builds, validates, or execs tool launch specifications.
- Rewrite the bundled skill's "打开 CLI" guidance: for a new task, preview with `--repo`, obtain approval, execute create, then run a no-`--repo` `create --dry-run`; after every repository reports `reuse`, the agent reads `taskflow.yaml`, composes a shell-appropriate native command line (first repository worktree as cwd, `--add-dir` for later worktrees and the task root, `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1` for Claude, safely quoted absolute paths), and presents it for the user to execute.
- Keep the `--worktree`/`--worktree=...` warning as skill-level guidance only; there is no runtime enforcement after removal.
- Update README and the skill content test to match the new guidance.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `development-tool-sessions`: Rewrite from "CLI builds and execs launch specifications" to "the skill composes native `claude`/`codex` command lines for the user to run"; remove CLI launch, PATH resolution, env injection, and argument-filtering requirements.
- `taskflow-multirepo-skill`: Replace `taskflow open` invocations with native command-line generation guidance gated on the post-create no-`--repo` `create --dry-run` reuse status.
- `readiness-and-initialization-integrity`: Remove the Open source-identity requirement while retaining the create non-bare-source preflight.
- `cli-output-contract`: Supported operational commands become create and delete.
- `e2e-command-flow`: Remove open preflight and binary open scenarios; keep create/delete coverage.
- `reporting-validation-readiness`: Operational result reporting covers create and delete only.
- `cli-operational-safety`: Remove open-side worktree identity and dirty-launch requirements; keep create-side reuse checks.
- `worktree-reconciliation`: Remove the state-free open readiness gate requirement's launch scenarios; keep state-free reuse semantics for create.
- `cross-task-source-coordination`: Remove open from the non-contending read-only command list.
- `aggregate-status-validation`: Reference create/delete action facts instead of create/open/delete.
- `task-configuration-validation`: Drop "open uses the first repository as cwd" phrasing; configuration order still defines the skill's cwd.

## Impact

- Code: `cmd/root.go` (delete open command), `internal/app/app.go` (delete `Open`/`preflightOpen`), delete `internal/devtool/` package.
- Tests: `cmd/root_test.go` command whitelist, `cmd/e2e_safety_test.go` open e2e, `internal/app/app_test.go` open cases, `skills/skill_content_test.go` content assertions.
- Docs: `README.md` open sections, `skills/taskflow/SKILL.md` 打开 CLI chapter and failure list.
- Users: `taskflow open` invocations stop working; the skill instead prints the native command to run.
