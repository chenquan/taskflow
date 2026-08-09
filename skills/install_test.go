package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallCreatesEverySkillForBothTools(t *testing.T) {
	root := t.TempDir()
	targets := []Target{
		{Tool: "codex", Root: filepath.Join(root, "codex", "skills")},
		{Tool: "claude", Root: filepath.Join(root, "claude", "skills")},
	}
	result, err := Install(targets, false)
	if err != nil {
		t.Fatal(err)
	}
	names, err := Names()
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != len(targets) || len(names) != 1 || names[0] != "taskflow" {
		t.Fatalf("unexpected install result %#v, names %#v", result, names)
	}
	for _, target := range targets {
		for _, name := range names {
			if _, err := os.Stat(filepath.Join(target.Root, name, "SKILL.md")); err != nil {
				t.Fatalf("%s %s not installed: %v", target.Tool, name, err)
			}
		}
	}
}

func TestInstallRejectsConflictWithoutChangingOtherTarget(t *testing.T) {
	root := t.TempDir()
	codexRoot := filepath.Join(root, "codex", "skills")
	claudeRoot := filepath.Join(root, "claude", "skills")
	conflict := filepath.Join(codexRoot, "taskflow")
	if err := os.MkdirAll(conflict, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(conflict, "SKILL.md"), []byte("custom"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install([]Target{{Tool: "codex", Root: codexRoot}, {Tool: "claude", Root: claudeRoot}}, false); err == nil {
		t.Fatal("expected conflict")
	}
	content, err := os.ReadFile(filepath.Join(conflict, "SKILL.md"))
	if err != nil || string(content) != "custom" {
		t.Fatalf("conflict changed: %q (%v)", content, err)
	}
	if _, err := os.Stat(claudeRoot); !os.IsNotExist(err) {
		t.Fatalf("other target should be unchanged, got %v", err)
	}
}

func TestInstallForceReplacesExistingSkill(t *testing.T) {
	root := t.TempDir()
	target := Target{Tool: "codex", Root: filepath.Join(root, "codex", "skills")}
	conflict := filepath.Join(target.Root, "taskflow")
	if err := os.MkdirAll(conflict, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(conflict, "custom.txt"), []byte("custom"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install([]Target{target}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(conflict, "custom.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale file preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(conflict, "SKILL.md")); err != nil {
		t.Fatalf("built-in skill absent: %v", err)
	}
}

func TestInstallRejectsMissingTargets(t *testing.T) {
	if _, err := Install(nil, false); err == nil {
		t.Fatal("expected empty target error")
	}
	for _, target := range []Target{{Root: t.TempDir()}, {Tool: "codex"}} {
		if _, err := Install([]Target{target}, false); err == nil {
			t.Fatalf("expected invalid target error for %#v", target)
		}
	}
}

func TestCleanupStagesAndRollback(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "stage")
	if err := os.Mkdir(stage, 0755); err != nil {
		t.Fatal(err)
	}
	cleanupStages([]replacement{{stage: stage}})
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatalf("stage was not cleaned: %v", err)
	}

	target := filepath.Join(root, "target")
	backup := target + ".taskflow-backup"
	if err := os.WriteFile(backup, []byte("backup"), 0644); err != nil {
		t.Fatal(err)
	}
	rollback([]replacement{{target: target, backup: backup}})
	if content, err := os.ReadFile(target); err != nil || string(content) != "backup" {
		t.Fatalf("rollback target=%q err=%v", content, err)
	}

	plainTarget := filepath.Join(root, "plain")
	if err := os.WriteFile(plainTarget, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	rollback([]replacement{{target: plainTarget}})
	if _, err := os.Stat(plainTarget); !os.IsNotExist(err) {
		t.Fatalf("plain target was not removed: %v", err)
	}
}
