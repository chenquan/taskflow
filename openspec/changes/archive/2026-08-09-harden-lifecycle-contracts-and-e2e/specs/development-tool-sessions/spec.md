## MODIFIED Requirements

### Requirement: Build safe Codex and Claude launch specifications
The CLI SHALL support enabled `open <task-id> --tool codex|claude` tools, use the configured executable and the primary worktree as cwd, add every secondary worktree and task root as additional directories, and MUST NOT include permission-bypass flags or Claude `--worktree`. A requested disabled or undefined tool MUST fail before acquiring a session lease.

#### Scenario: Build Codex launch
- **WHEN** the user opens an enabled Codex tool with a custom executable
- **THEN** the process uses that executable, the primary worktree cwd, and repeated `--add-dir` arguments for other managed directories

#### Scenario: Enable Claude additional instructions
- **WHEN** Claude configuration enables additional instructions
- **THEN** the launch environment includes `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`

#### Scenario: Reject a disabled tool
- **WHEN** a user requests a known tool that is not listed in `enabled_tools`
- **THEN** open returns a configuration error without creating a session lease or starting a process

### Requirement: Report child failures under stable exit codes
A development-tool process that exits non-zero MUST release its session lease, return specflow execution exit code 1, and include the actual child exit code, tool ID, and executable in result data.

#### Scenario: Codex exits with a custom code
- **WHEN** the configured Codex executable exits with code 23
- **THEN** specflow exits with code 1, reports `childExitCode: 23`, and a subsequent tool session can start
