package skills

import (
	"strings"
	"testing"
)

func TestTaskflowSkillGuidesCreateOpenAndDelete(t *testing.T) {
	content, err := Files.ReadFile("taskflow/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, required := range []string{
		"create <task-id>",
		"create <task-id> --execute",
		"open <task-id>",
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
