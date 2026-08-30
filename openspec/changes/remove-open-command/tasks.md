## 1. Remove the Go launch path

- [x] 1.1 Delete the `open` command block and its `--tool` flag from `cmd/root.go`
- [x] 1.2 Delete `Service.Open` and `preflightOpen` from `internal/app/app.go`
- [x] 1.3 Delete the `internal/devtool` package (`adapter.go` and `adapter_test.go`)
- [x] 1.4 Verify `go build ./...` and `go vet ./...` pass with no orphaned references

## 2. Update tests

- [x] 2.1 Update the command whitelist test in `cmd/root_test.go` to expect create, delete, version, and skill only
- [x] 2.2 Rework the open e2e case in `cmd/e2e_safety_test.go` into a create --dry-run reuse assertion on a dirty matching worktree
- [x] 2.3 Remove the open launch cases from `internal/app/app_test.go`
- [x] 2.4 Run `go test ./...` and fix any remaining open references

## 3. Rewrite the skill guidance

- [x] 3.1 Rewrite the "打开 CLI" chapter of `skills/taskflow/SKILL.md`: gate on `create --dry-run` reporting every repository as `reuse`, then compose the native `claude`/`codex` command (first worktree as cwd, absolute `--add-dir` paths for later worktrees and the task root, `CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1` prefix for Claude) and present it for the user to run; keep the `--worktree` warning
- [x] 3.2 Remove open references from the SKILL.md intro, the taskflow.yaml note, and the `TOOL_NOT_FOUND` entry in the failure list
- [x] 3.3 Update `skills/skill_content_test.go`: replace the `"open <task-id>"` assertion with assertions for the composed native commands and the dry-run reuse gate

## 4. Update documentation

- [x] 4.1 Update `README.md`: remove open usage examples and behavior notes, document the skill-composed native command replacement, and adjust the create/open/delete lifecycle phrasing

## 5. Final verification

- [x] 5.1 Run `go test ./...`, `go vet ./...`, and `openspec validate remove-open-command`; confirm all pass
