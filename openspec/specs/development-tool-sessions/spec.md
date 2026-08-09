## Purpose

Define safe configured Codex and Claude launches from managed primary worktrees with controlled additional directories.

## Requirements

### Requirement: Build safe Codex and Claude launch specifications
The CLI SHALL support configured `open <task-id> [--tool codex|claude]` tools, use `development.default_tool` when omitted, use the configured executable and the primary worktree as cwd, add every secondary worktree and task root as additional directories, and MUST NOT include Claude `--worktree`. Extra tool arguments, including permission-bypass flags, MUST be forwarded unchanged. An unsupported or undefined tool MUST fail before launching. Claude MAY receive `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1` when configured.

#### Scenario: Build Codex launch
- **WHEN** the user opens a configured Codex tool with a custom executable
- **THEN** the process uses that executable, the primary worktree cwd, and repeated `--add-dir` arguments for other managed directories

#### Scenario: Enable Claude additional instructions
- **WHEN** Claude configuration enables additional instructions
- **THEN** the launched child receives `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`
