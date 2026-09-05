//go:build unix

package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestE2EBuiltBinaryRetriesFailedSourceCopy(t *testing.T) {
	repo := e2eGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "a-first.env"), []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(repo, "m-pipe"), 0644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "z-last.env"), []byte("last"), 0600); err != nil {
		t.Fatal(err)
	}
	tasks := t.TempDir()
	binary := filepath.Join(t.TempDir(), "taskflow")
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

	failing := exec.Command(binary, "--tasks-root", tasks, "--json", "create", "BINARY-RETRY", "--repo", "app="+repo, "--execute")
	output, err := failing.CombinedOutput()
	if err == nil {
		t.Fatalf("expected copy failure, output=%s", output)
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 4 {
		t.Fatalf("expected partial exit code 4, got %v output=%s", err, output)
	}
	var envelope struct {
		OK     bool `json:"ok"`
		Errors []struct {
			Code string `json:"code"`
			Repo string `json:"repo"`
		} `json:"errors"`
		Data struct {
			Actions []struct {
				Kind   string `json:"kind"`
				Status string `json:"status"`
			} `json:"actions"`
		} `json:"data"`
	}
	// The binary prints the JSON envelope first and cobra's error line after;
	// decode only the leading JSON value.
	decoder := json.NewDecoder(bytes.NewReader(output))
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("unparseable failure output: %v: %s", err, output)
	}
	if envelope.OK || len(envelope.Errors) != 1 || envelope.Errors[0].Code != "SOURCE_COPY_UNSUPPORTED_ENTRY" || envelope.Errors[0].Repo != "app" {
		t.Fatalf("failure envelope: %#v", envelope)
	}
	if len(envelope.Data.Actions) != 2 || envelope.Data.Actions[0].Status != "created" || envelope.Data.Actions[1].Status != "failed" {
		t.Fatalf("failure actions: %#v", envelope.Data.Actions)
	}
	target := filepath.Join(tasks, "BINARY-RETRY", "worktrees", "app")
	if _, statErr := os.Stat(filepath.Join(target, "a-first.env")); statErr != nil {
		t.Fatalf("entries before the failure were not copied: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(target, "z-last.env")); !os.IsNotExist(statErr) {
		t.Fatalf("entries after the failure were copied: %v", statErr)
	}

	if err := os.Remove(filepath.Join(repo, "m-pipe")); err != nil {
		t.Fatal(err)
	}
	retryOutput, err := exec.Command(binary, "--tasks-root", tasks, "--json", "create", "BINARY-RETRY", "--execute").CombinedOutput()
	if err != nil {
		t.Fatalf("retry: %v: %s", err, retryOutput)
	}
	if err := json.Unmarshal(retryOutput, &envelope); err != nil {
		t.Fatalf("unparseable retry output: %v: %s", err, retryOutput)
	}
	if !envelope.OK || len(envelope.Errors) != 0 {
		t.Fatalf("retry envelope: %#v", envelope)
	}
	if len(envelope.Data.Actions) != 2 || envelope.Data.Actions[0].Status != "reuse" || envelope.Data.Actions[1].Status != "repaired" {
		t.Fatalf("retry actions: %#v", envelope.Data.Actions)
	}
	if _, statErr := os.Stat(filepath.Join(target, "z-last.env")); statErr != nil {
		t.Fatalf("retry did not complete the source copy: %v", statErr)
	}
}
