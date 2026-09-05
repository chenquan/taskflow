//go:build unix

package fsx

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCopyTreeFailsOnUnsupportedEntry(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(filepath.Join(source, "before.txt"), []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(source, "pipe"), 0644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	stats, err := CopyTree(source, target)
	var copyErr *CopyError
	if !errors.As(err, &copyErr) || copyErr.Op != "unsupported-entry" || copyErr.Path != "pipe" {
		t.Fatalf("expected unsupported-entry error for pipe, got stats=%#v err=%v", stats, err)
	}
	if _, statErr := os.Stat(filepath.Join(target, "before.txt")); statErr != nil {
		t.Fatalf("entries before the failure were not copied: %v", statErr)
	}
}
