## MODIFIED Requirements

### Requirement: Build safe Codex and Claude launch specifications
The CLI SHALL support `open <task-id> [--tool codex|claude] [-- <tool-args>...]`, default to Codex when the tool is omitted, resolve the selected built-in executable from `PATH`, use the first repository worktree as cwd, and add every later repository worktree and the task root as additional directories. Extra tool arguments, including permission-bypass flags, MUST be forwarded unchanged, except `--worktree` and `--worktree=...` MUST be rejected before launch. Claude MUST receive `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`. Unsupported tools and missing executables MUST fail before launch.

#### Scenario: Build Codex launch
- **WHEN** the user opens Codex for a ready task with multiple repositories
- **THEN** the process uses `codex` from `PATH`, the first repository worktree as cwd, and repeated `--add-dir` arguments for remaining worktrees and task root

#### Scenario: Enable Claude additional instructions
- **WHEN** the user opens Claude for a ready task
- **THEN** the launched child uses `claude` from `PATH` and receives `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`

#### Scenario: Forward safe tool arguments
- **WHEN** the user provides arguments after `--` that do not request a nested worktree
- **THEN** the selected tool receives those arguments unchanged after Taskflow's additional-directory arguments

#### Scenario: Reject nested worktree arguments
- **WHEN** forwarded arguments contain `--worktree` or a `--worktree=...` value
- **THEN** open returns an invalid-argument diagnostic without launching the child

## ADDED Requirements

### Requirement: Launch only from a structurally ready workspace
Before launching a tool, open MUST verify every configured worktree exists, belongs to the configured source repository, and uses the expected branch. It MUST NOT require a state file, phase, digest, or prior Taskflow action outcome. Dirty matching worktrees MUST remain launchable.

#### Scenario: Reject a missing or mismatched worktree
- **WHEN** a configured worktree is missing, belongs to another source, or uses an unexpected branch
- **THEN** open returns a worktree diagnostic without launching a tool

#### Scenario: Open a dirty ready workspace
- **WHEN** every configured worktree is valid and at least one contains uncommitted changes
- **THEN** open launches the selected tool with all configured directories
