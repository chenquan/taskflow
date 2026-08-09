## Purpose

Define safe configured development-tool launches.

## Requirements

### Requirement: Build safe Codex and Claude launch specifications
The CLI SHALL support enabled `open <task-id> --tool codex|claude` tools, use the configured executable and the primary worktree as cwd, add every secondary worktree and task root as additional directories, and MUST NOT include permission-bypass flags or Claude `--worktree`. A requested disabled or undefined tool MUST fail before launching. The launched child environment MUST contain at most one effective value for each environment key, with configured adapter values overriding inherited values and Windows key comparison treated case-insensitively.

#### Scenario: Build Codex launch
- **WHEN** the user opens an enabled Codex tool with a custom executable
- **THEN** the process uses that executable, the primary worktree cwd, and repeated `--add-dir` arguments for other managed directories

#### Scenario: Enable Claude additional instructions
- **WHEN** Claude configuration enables additional instructions and the inherited environment already defines that key
- **THEN** the launched child receives exactly one `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1` value
