//go:build unix

package app

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/chenquan/taskflow/internal/ownership"
	"github.com/chenquan/taskflow/internal/plan"
	"github.com/chenquan/taskflow/internal/report"
)

func TestCreateRetainsPendingSourceCopyAfterPartialFailure(t *testing.T) {
	repo := makeGitRepo(t)
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
	result, code := New().Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "PARTIAL", Repositories: []string{"app=" + repo}, Execute: true})
	if code != report.ExitPartial || result.OK || !hasDiagnostic(result.Errors, "SOURCE_COPY_UNSUPPORTED_ENTRY") {
		t.Fatalf("partial copy: code=%d result=%#v", code, result)
	}
	items := result.Data.(map[string]any)["actions"].([]plan.Item)
	if len(items) != 2 || items[0].Status != "created" || items[1].Status != "failed" {
		t.Fatalf("partial copy actions: %#v", items)
	}
	target := filepath.Join(tasks, "PARTIAL", "worktrees", "app")
	if _, err := os.Stat(filepath.Join(target, "a-first.env")); err != nil {
		t.Fatalf("entries before the failure were not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "z-last.env")); !os.IsNotExist(err) {
		t.Fatalf("entries after the failure were copied: %v", err)
	}
	manifest, exists, err := ownership.Load(filepath.Join(tasks, "PARTIAL"))
	if err != nil || !exists || manifest.Worktrees[0].SourceCopy == nil || manifest.Worktrees[0].SourceCopy.Status != "pending" {
		t.Fatalf("pending ownership after partial copy: manifest=%#v exists=%v err=%v", manifest, exists, err)
	}
	if err := os.Remove(filepath.Join(repo, "m-pipe")); err != nil {
		t.Fatal(err)
	}
	retry, code := New().Create(context.Background(), CreateOptions{TasksRoot: tasks, TaskID: "PARTIAL", Execute: true})
	if code != report.ExitOK || !retry.OK {
		t.Fatalf("retry: code=%d result=%#v", code, retry)
	}
	retryItems := retry.Data.(map[string]any)["actions"].([]plan.Item)
	if len(retryItems) != 2 || retryItems[0].Status != "reuse" || retryItems[1].Status != "repaired" {
		t.Fatalf("retry actions: %#v", retryItems)
	}
	if _, err := os.Stat(filepath.Join(target, "z-last.env")); err != nil {
		t.Fatalf("retry did not complete the source copy: %v", err)
	}
	manifest, _, err = ownership.Load(filepath.Join(tasks, "PARTIAL"))
	if err != nil || manifest.Worktrees[0].SourceCopy.Status != "complete" {
		t.Fatalf("source-copy status after retry: %#v err=%v", manifest, err)
	}
}
