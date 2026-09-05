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

func TestE2EBuiltBinaryReportsSourceCopyAction(t *testing.T) {
	repo := e2eGitRepo(t)
	localName := "local settings.env"
	if runtime.GOOS == "windows" {
		localName = "local settings.env"
	}
	if err := os.WriteFile(filepath.Join(repo, localName), []byte("PORT=4310\n"), 0600); err != nil {
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
	preview := run("--tasks-root", tasks, "--json", "create", "BINARY", "--repo", "app="+repo, "--dry-run")
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
			Source    string `json:"source"`
			Target    string `json:"target"`
			FileCount int    `json:"fileCount"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Actions) != 2 || data.Actions[0].Kind != "worktree" || data.Actions[0].Status != "create" || data.Actions[1].Kind != "source-copy" || data.Actions[1].Status != "copy" {
		t.Fatalf("binary source-copy preview: %#v", data.Actions)
	}
	if data.Actions[1].Source == "" || filepath.Base(data.Actions[1].Source) != "repo" || !strings.HasSuffix(data.Actions[1].Target, filepath.Join("BINARY", "worktrees", "app")) {
		t.Fatalf("source-copy action paths: %#v", data.Actions[1])
	}
	textPreview := run("--tasks-root", tasks, "create", "BINARY", "--repo", "app="+repo, "--dry-run")
	encodedRepo, err := json.Marshal(repo)
	if err != nil {
		t.Fatal(err)
	}
	encodedRepoText := string(encodedRepo[1 : len(encodedRepo)-1])
	if !strings.Contains(string(textPreview), "COPY source") || !strings.Contains(string(textPreview), encodedRepoText) {
		t.Fatalf("binary text source-copy preview: %s", textPreview)
	}
	run("--tasks-root", tasks, "--json", "create", "BINARY", "--repo", "app="+repo, "--execute")
	copied, err := os.ReadFile(filepath.Join(tasks, "BINARY", "worktrees", "app", localName))
	if err != nil || string(copied) != "PORT=4310\n" {
		t.Fatalf("binary source copy is incomplete: %q err=%v", copied, err)
	}
	if _, err := os.Stat(filepath.Join(tasks, "BINARY", "worktrees", "app", ".git")); err != nil {
		t.Fatalf("binary worktree lost its git metadata: %v", err)
	}
	repeat := run("--tasks-root", tasks, "--json", "create", "BINARY", "--dry-run")
	if !strings.Contains(string(repeat), `"status": "reuse"`) || !strings.Contains(string(repeat), `"kind": "source-copy"`) {
		t.Fatalf("binary repeat did not report source-copy reuse: %s", repeat)
	}
	executeItems := run("--tasks-root", tasks, "--json", "create", "BINARY", "--execute")
	var executeEnvelope struct {
		Data struct {
			Actions []struct {
				Kind   string `json:"kind"`
				Status string `json:"status"`
			} `json:"actions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(executeItems, &executeEnvelope); err != nil {
		t.Fatal(err)
	}
	if len(executeEnvelope.Data.Actions) != 2 || executeEnvelope.Data.Actions[0].Status != "reuse" || executeEnvelope.Data.Actions[1].Status != "reuse" {
		t.Fatalf("binary repeat execute actions: %#v", executeEnvelope.Data.Actions)
	}
}
