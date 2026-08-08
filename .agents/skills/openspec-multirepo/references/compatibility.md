# Compatibility reference

Codex uses repeated `--add-dir` for secondary worktrees and the task root. Claude uses the same directory grants and may set `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1`. Neither tool receives permission bypass flags or Claude `--worktree`.
