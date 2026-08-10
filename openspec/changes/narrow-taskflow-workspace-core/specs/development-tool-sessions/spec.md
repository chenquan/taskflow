## MODIFIED Requirements

### Requirement: Build safe Codex and Claude launch specifications
The CLI SHALL support `open <task-id> [--tool codex|claude] [-- <tool-args>...]`, default to Codex when the tool is omitted, resolve the selected built-in executable from `PATH`, use the first repository worktree as cwd, and add every later repository worktree and the task root as additional directories. Extra tool arguments, including permission-bypass flags, MUST be forwarded unchanged, except `--worktree` and `--worktree=...` MUST be rejected before launch. Claude MUST receive `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`. Unsupported tools and missing executables MUST fail before launch.

#### Scenario: Build Codex launch
- **WHEN** the user opens Codex for a started task with multiple repositories
- **THEN** the process uses `codex` from `PATH`, the first repository worktree cwd, and repeated `--add-dir` arguments for the remaining worktrees and task root

#### Scenario: Enable Claude additional instructions
- **WHEN** the user opens Claude for a started task
- **THEN** the launched child uses `claude` from `PATH` and receives `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`

#### Scenario: Forward safe tool arguments
- **WHEN** the user provides arguments after `--` that do not request a nested worktree
- **THEN** the selected tool receives those arguments unchanged after Taskflow's additional-directory arguments

#### Scenario: Reject nested worktree arguments
- **WHEN** forwarded arguments contain `--worktree` or a `--worktree=...` value
- **THEN** open returns an invalid-argument diagnostic without launching the child

## ADDED Requirements

### Requirement: Launch only from a ready managed workspace
Before launching a tool, `open` MUST require schema-v2 task state in phase `started` and MUST verify that every configured worktree exists, belongs to the configured source repository, and uses the expected branch. Dirty managed worktrees MUST remain launchable.

#### Scenario: Reject an unstarted task
- **WHEN** a task is initialized, starting, or failed
- **THEN** open returns `WORKSPACE_NOT_STARTED` without launching a tool

#### Scenario: Reject a missing or mismatched worktree
- **WHEN** a started task has a missing worktree, a worktree from another source, or an unexpected branch
- **THEN** open returns a worktree diagnostic without launching a tool

#### Scenario: Open a dirty ready workspace
- **WHEN** every configured worktree is valid and at least one contains uncommitted changes
- **THEN** open launches the selected tool with all managed directories
