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
	if len(result) != len(targets) || len(names) != 1 || names[0] != "specflow" {
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
	conflict := filepath.Join(codexRoot, "specflow")
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
	conflict := filepath.Join(target.Root, "specflow")
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
