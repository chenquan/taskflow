package skills

import (
	"strings"
	"testing"
)

func TestTaskflowSkillGuidesTheUserThroughTheLifecycle(t *testing.T) {
	content, err := Files.ReadFile("taskflow/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}

	text := string(content)
	for _, required := range []string{
		"status <task-id>",
		"若用户没有提供 `task-id`，先询问任务 ID",
		"任务目录存在但 `taskflow.yaml` 缺失",
		"这是预期状态，运行 `start --dry-run`",
		"init <task-id>",
		"start <task-id> --dry-run",
		"start <task-id> --execute",
		"open <task-id>",
		"validate <task-id>",
		"repo add <task-id>",
		"STATE_CONFLICT",
		"WORKTREE_MISMATCH",
		"SOURCE_LOCK_UNAVAILABLE",
		"REPO_ADD_WRITE_FAILED",
		"VALIDATION_*",
		"--worktree",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("skill is missing %q", required)
		}
	}

	for _, forbidden := range []string{
		"inventory.json",
		"contract owners",
		"角色负责人",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("skill contains removed or unsupported guidance %q", forbidden)
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
