package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestE2EBuiltBinaryPreservesOverlayActionData(t *testing.T) {
	repo := e2eGitRepo(t)
	localName := "local\nsettings.env"
	displayName := "local\\nsettings.env"
	if runtime.GOOS == "windows" {
		localName = "local settings.env"
		displayName = localName
	}
	localPath := filepath.Join(repo, localName)
	if err := os.WriteFile(localPath, []byte("PORT=4310\n"), 0600); err != nil {
		t.Fatal(err)
	}
	tasks := t.TempDir()
	binaryName := "taskflow"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(t.TempDir(), binaryName)
	moduleRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	moduleRoot = filepath.Dir(moduleRoot)
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build taskflow: %v: %s", err, output)
	}
	run := func(args ...string) []byte {
		t.Helper()
		command := exec.Command(binary, args...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("taskflow %v: %v: %s", args, err, output)
		}
		return output
	}
	preview := run("--tasks-root", tasks, "--json", "create", "BINARY", "--repo", "app="+repo, "--local", "app="+localName, "--dry-run")
	var envelope struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(preview, &envelope); err != nil || !envelope.OK {
		t.Fatalf("binary preview: err=%v output=%s", err, preview)
	}
	var data struct {
		Actions []struct {
			Kind      string `json:"kind"`
			Status    string `json:"status"`
			FileCount int    `json:"fileCount"`
			Files     []struct {
				Path string `json:"path"`
			} `json:"files"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Actions) != 2 || data.Actions[0].Kind != "worktree" || data.Actions[0].Status != "create" || data.Actions[1].Kind != "overlay" || data.Actions[1].Status != "copy" || data.Actions[1].FileCount != 1 || len(data.Actions[1].Files) != 1 || data.Actions[1].Files[0].Path != localName {
		t.Fatalf("binary overlay preview: %#v", data.Actions)
	}
	textPreview := run("--tasks-root", tasks, "create", "BINARY", "--repo", "app="+repo, "--local", "app="+localName, "--dry-run")
	if !strings.Contains(string(textPreview), "COPY local overlay") || !strings.Contains(string(textPreview), displayName) {
		t.Fatalf("binary text overlay preview: %s", textPreview)
	}
	invalid := exec.Command(binary, "--tasks-root", tasks, "--json", "create", "INVALID", "--repo", "app="+repo, "--local", "app=../outside", "--dry-run")
	invalidOutput, invalidErr := invalid.CombinedOutput()
	if invalidErr == nil || invalid.ProcessState.ExitCode() != 2 || !strings.Contains(string(invalidOutput), "INVALID_CONFIGURATION") {
		t.Fatalf("binary invalid overlay: err=%v exit=%d output=%s", invalidErr, invalid.ProcessState.ExitCode(), invalidOutput)
	}
	run("--tasks-root", tasks, "--json", "create", "BINARY", "--repo", "app="+repo, "--local", "app="+localName, "--execute")
	if _, err := os.Stat(filepath.Join(tasks, "BINARY", "worktrees", "app", localName)); err != nil {
		t.Fatalf("binary overlay file missing: %v", err)
	}
	repeat := run("--tasks-root", tasks, "--json", "create", "BINARY", "--dry-run")
	if !strings.Contains(string(repeat), `"status": "reuse"`) || !strings.Contains(string(repeat), `"kind": "overlay"`) {
		t.Fatalf("binary repeat did not report reuse overlay: %s", repeat)
	}
}
