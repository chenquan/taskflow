## Context

Taskflow currently exposes `taskflow open <task-id> [--tool codex|claude] [-- <tool-args>...]`, implemented by `cmd/root.go` (Cobra command), `Service.Open`/`preflightOpen` in `internal/app/app.go`, and `internal/devtool` (launch-spec builder). The command resolves the tool from `PATH`, runs a structural worktree preflight, assembles `--add-dir` arguments, injects `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1` for Claude, rejects `--worktree` arguments, and streams the child process in the current terminal.

The bundled skill (`skills/taskflow/SKILL.md`) is what agents actually follow. An agent can compose the equivalent native `claude`/`codex` command line from `taskflow.yaml` and hand it to the user, which removes the need for a Go launch path entirely. Eleven main specs reference `open`, so this is a cross-cutting contract change.

## Goals / Non-Goals

**Goals:**
- Remove the `open` command, `Service.Open`, `preflightOpen`, and the `internal/devtool` package.
- Move tool-launch knowledge (cwd selection, `--add-dir` assembly, Claude env var, `--worktree` warning) into the skill as guidance for composing native commands that the user executes.
- Preserve the "launch only from a structurally ready workspace" semantic by gating the composed command on `create --dry-run` reporting every repository as `reuse`.
- Update all eleven affected specs, README, and tests.

**Non-Goals:**
- No new CLI command replaces `open` (no `taskflow claude`/`taskflow codex`).
- No compatibility or deprecation period for `open`; this project consistently ships breaking CLI changes documented in the README.
- No runtime enforcement of composed command safety (no `--worktree` rejection code, no PATH checks) — the user's own shell surfaces those errors.
- No changes to create, delete, lock, ownership, or configuration behavior.

## Decisions

### 1. Delete the entire launch path rather than keep it internal

`devtool` is only referenced by `Service.Open`; keeping it as internal-only code would leave an uncalled module. Delete `cmd/root.go` open block, `Service.Open`, `preflightOpen`, and `internal/devtool/` wholesale.

### 2. Skill composes the command; the user executes it

The skill instructs the agent to read `taskflow.yaml` and present a ready-to-paste command instead of exec-ing the tool. Rationale: an agent shell typically has no PTY for an interactive TUI, and the user may want to add model/permission flags before launching. The skill first identifies the user's shell and quotes/escapes every path for POSIX shell, PowerShell, or cmd.exe; it composes:

- `cd <task-root>/worktrees/<first-repo-worktree>` (first repository stays the cwd/primary)
- `claude --add-dir <abs-path-per-later-worktree>... --add-dir <abs-task-root>` prefixed with `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`
- `codex --add-dir <abs-path-per-later-worktree>... --add-dir <abs-task-root>`
- absolute paths only, so the pasted command works from any directory; shell-specific quoting keeps spaces and metacharacters from changing the command

### 3. `create --dry-run` is the readiness gate

`preflightOpen`'s checks (target exists, belongs to source common dir, expected branch; dirty is acceptable) are a subset of what create's reuse classification already verifies. For a new task, the skill first previews the requested repositories, obtains execute approval, runs create, and then runs `taskflow create <task-id> --dry-run` without `--repo`; only when this post-create check reports `reuse` for every repository (no `create`, no conflict) should the agent compose and present the tool command. Existing tasks start with the same no-`--repo` check. Zero new Go code; the "structurally ready" requirement keeps a deterministic check.

### 4. Safety notes degrade from runtime enforcement to skill guidance

`--worktree`/`--worktree=...` rejection and `TOOL_NOT_FOUND` handling disappear with the launch path. The skill keeps the `--worktree` warning as text (the skill content test already asserts it). A missing tool binary is reported by the user's own shell, so no diagnostic replaces `TOOL_NOT_FOUND`.

### 5. Spec cleanup rides the same delta

While rewriting `cli-output-contract`'s exit-code requirement, drop "1 for ... a launched child process that exits non-zero" and "6 for external-tool incompatibility" — both describe the removed launch path (the `ExitToolCompatibility` constant is pre-existing dead code and stays untouched).

## Risks / Trade-offs

- [Readiness gate is advisory] An agent may skip the `create --dry-run` step and compose a command for a broken workspace → the skill text makes the dry-run gate the first instruction of the launch flow; the launched tool itself will also surface a bad worktree.
- [Composed commands are unvalidated] A wrong path or typo'd flag in the generated command is no longer caught by Go code → absolute-path rule in the skill; the user reviews the command before running it.
- [Eleven-spec blast radius] Deltas touch many requirement texts → most edits are sentence-level removals; `development-tool-sessions`, `taskflow-multirepo-skill`, and `readiness-and-initialization-integrity` carry the substantive rewrites.
- [Breaking change for existing users] `taskflow open` invocations fail with "unknown command" → README documents the native-command replacement; the skill (re)installation path ships the new guidance.

## Migration Plan

No runtime migration. Users replace `taskflow open <id> [--tool ...]` with the skill-composed native command; README shows the exact mapping. Rollback is a git revert; no persisted state is affected.
