## Purpose

Define safe configured development-tool launches and exclusive task sessions.

## Requirements

### Requirement: Build safe Codex and Claude launch specifications
The CLI SHALL support `open <task-id> --tool codex|claude`, use the primary worktree as cwd, add every secondary worktree and task root as additional directories, and MUST NOT include permission-bypass flags or Claude `--worktree`.

#### Scenario: Build Codex launch
- **WHEN** the user opens a task with Codex
- **THEN** the process uses the primary worktree cwd and repeated `--add-dir` arguments for other managed directories

#### Scenario: Enable Claude additional instructions
- **WHEN** Claude configuration enables additional instructions
- **THEN** the launch environment includes `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`

### Requirement: Serialize active write sessions
The CLI MUST hold a session lease for the entire child process and reject another `open` for the same task while the recorded PID is live.

#### Scenario: Concurrent tool open
- **WHEN** Codex is active and a user starts Claude for the same task
- **THEN** the second command returns a session conflict without starting Claude
