package skills

import (
	"strings"
	"testing"
)

func TestTaskflowSkillGuidesCreateNativeToolsAndDelete(t *testing.T) {
	content, err := Files.ReadFile("taskflow/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, required := range []string{
		"create <task-id>",
		"create <task-id> --execute",
		"delete <task-id>",
		"ownership.json",
		"OWNERSHIP_NOT_FOUND",
		"taskflow.yaml",
		"dry-run",
		"SOURCE_BRANCH_LOCKED",
		"WORKTREE_MISMATCH",
		"CREATE_WORKTREE_FAILED",
		"CONFIG_EDIT_REQUIRED",
		"直接编辑 taskflow.yaml",
		"create --dry-run",
		"reuse",
		"claude",
		"codex",
		"--add-dir",
		"CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1",
		"绝对路径",
		"execute 完成后",
		"PowerShell",
		"cmd.exe",
		"Set-Location -LiteralPath",
		"$env:CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD",
		"cd /d",
		"set \"CLAUDE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD=1\"",
		"单引号",
		"转义",
		"--worktree",
		"dirty",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("skill is missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"status <task-id>",
		"validate <task-id>",
		"repo add <task-id>",
		"state.json",
		"inventory.json",
		"STATE_CONFLICT",
		"depends_on",
		"repo add",
		"open <task-id>",
		"TOOL_NOT_FOUND",
		"TOOL_EXITED",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("skill contains retired guidance %q", forbidden)
		}
	}
}

func TestTaskflowSkillMetadataMatchesTheGuidance(t *testing.T) {
	content, err := Files.ReadFile("taskflow/agents/openai.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, required := range []string{
		"Taskflow 工作区向导",
		"向导式安全管理多仓库 AI 工作区",
		"$taskflow",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("metadata is missing %q", required)
		}
	}
}
