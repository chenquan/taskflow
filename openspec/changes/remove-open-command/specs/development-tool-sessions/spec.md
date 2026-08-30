## ADDED Requirements

### Requirement: Compose native tool commands from skill guidance
The skill SHALL instruct an agent to compose ready-to-run native `claude` and `codex` command lines from taskflow.yaml for the user to execute, instead of launching tools through the CLI. The composed command MUST use the first repository worktree as the working directory, pass absolute additional-directory paths for every later repository worktree and the task root via `--add-dir`, and prefix Claude invocations with `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`. The agent MUST present the command to the user rather than execute it, and MUST warn against `--worktree` and `--worktree=...` arguments.

#### Scenario: Compose a Claude command
- **WHEN** every configured worktree reports `reuse` and the user requests Claude
- **THEN** the agent presents a command that changes into the first repository worktree, sets `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`, invokes `claude`, and passes `--add-dir` with absolute paths for every later worktree and the task root

#### Scenario: Compose a Codex command
- **WHEN** every configured worktree reports `reuse` and the user requests Codex
- **THEN** the agent presents a `codex` command with the same working directory and the same absolute `--add-dir` arguments

#### Scenario: Gate composition on structural readiness
- **WHEN** `create --dry-run` reports any repository as `create` or a conflict
- **THEN** the agent surfaces the reported issue instead of composing a tool command

#### Scenario: Warn against nested worktree flags
- **WHEN** the user requests `--worktree` or `--worktree=...` in the composed invocation
- **THEN** the agent omits or refuses the flag and explains the nested-worktree risk

## REMOVED Requirements

### Requirement: Build safe Codex and Claude launch specifications
**Reason**: The CLI no longer builds, validates, or execs tool launch specifications; launch knowledge moves into the skill as native command-line composition guidance.
**Migration**: Run the skill-composed native `claude`/`codex` command instead of `taskflow open`. The `--add-dir` assembly, Claude environment variable, and user tool arguments are expressed directly in the composed command line.

### Requirement: Launch only from a structurally ready workspace
**Reason**: With no CLI launch path there is no runtime preflight to enforce.
**Migration**: The skill gates command composition on `create --dry-run` reporting every repository as `reuse`, which performs the same live worktree identity checks; dirty matching worktrees remain launchable.
