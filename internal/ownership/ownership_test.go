package ownership

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	manifest := New("TASK-1")
	manifest.Add(Worktree{
		Repository: "api",
		Source:     filepath.Join(root, "source"),
		CommonDir:  filepath.Join(root, "source", ".git"),
		Branch:     "feature/task-1",
		Target:     filepath.Join(root, "task", "worktrees", "api"),
	})
	if err := Save(root, manifest); err != nil {
		t.Fatal(err)
	}
	loaded, exists, err := Load(root)
	if err != nil || !exists {
		t.Fatalf("loaded=%#v exists=%v err=%v", loaded, exists, err)
	}
	if loaded.TaskID != manifest.TaskID || len(loaded.Worktrees) != 1 || loaded.Worktrees[0] != manifest.Worktrees[0] {
		t.Fatalf("loaded=%#v want=%#v", loaded, manifest)
	}
}

func TestLoadMissingManifest(t *testing.T) {
	manifest, exists, err := Load(t.TempDir())
	if err != nil || exists || manifest.Version != 0 {
		t.Fatalf("manifest=%#v exists=%v err=%v", manifest, exists, err)
	}
}

func TestLoadRejectsUnsupportedVersion(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".taskflow"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root), []byte(`{"version":99,"taskID":"TASK"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(root); err == nil {
		t.Fatal("expected unsupported version error")
	}
}

func TestValidateRejectsMalformedEntries(t *testing.T) {
	valid := Worktree{Repository: "repo", Source: "/source", CommonDir: "/source/.git", Branch: "feature/task", Target: "/task/worktrees/repo"}
	cases := []Manifest{
		{Version: 99, TaskID: "TASK", Worktrees: []Worktree{valid}},
		{Version: Version, Worktrees: []Worktree{valid}},
		{Version: Version, TaskID: "TASK", Worktrees: []Worktree{{Repository: "repo"}}},
		{Version: Version, TaskID: "TASK", Worktrees: []Worktree{valid, valid}},
	}
	for index, manifest := range cases {
		if err := Validate(manifest); err == nil {
			t.Fatalf("case %d unexpectedly passed", index)
		}
	}
}

func TestSourceCopyOwnershipRoundTripAndLegacyCompatibility(t *testing.T) {
	root := t.TempDir()
	manifest := New("TASK-COPY")
	manifest.Add(Worktree{
		Repository: "api",
		Source:     filepath.Join(root, "source"),
		CommonDir:  filepath.Join(root, "source", ".git"),
		Branch:     "feature/task-copy",
		Target:     filepath.Join(root, "task", "worktrees", "api"),
		SourceCopy: &SourceCopy{
			Source: filepath.Join(root, "source"),
			Target: filepath.Join(root, "task", "worktrees", "api"),
			Status: "pending",
		},
	})
	if err := Save(root, manifest); err != nil {
		t.Fatal(err)
	}
	loaded, exists, err := Load(root)
	if err != nil || !exists || loaded.Worktrees[0].SourceCopy == nil || loaded.Worktrees[0].SourceCopy.Status != "pending" {
		t.Fatalf("source-copy manifest=%#v exists=%v err=%v", loaded, exists, err)
	}
	legacy := New("LEGACY")
	legacy.Add(Worktree{Repository: "api", Source: "/source", CommonDir: "/source/.git", Branch: "feature/legacy", Target: "/task/worktrees/api"})
	if err := Validate(legacy); err != nil {
		t.Fatalf("legacy manifest rejected: %v", err)
	}
}
