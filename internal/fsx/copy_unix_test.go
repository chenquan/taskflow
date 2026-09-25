//go:build unix

package fsx

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOverlayFailsOnUnsupportedCarriedEntry(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(filepath.Join(source, "carry"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "carry", "before.txt"), []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(source, "carry", "pipe"), 0644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "carry", "z-last.txt"), []byte("last"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(source, "ignored-pipe"), 0644); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	manifest := writeManifest(t, source, "carry/", "before.txt")
	stats, _, err := Overlay(source, target, manifest)
	var copyErr *CopyError
	if !errors.As(err, &copyErr) || copyErr.Op != "unsupported-entry" || copyErr.Path != filepath.Join("carry", "pipe") {
		t.Fatalf("expected unsupported-entry error for carried pipe, got stats=%#v err=%v", stats, err)
	}
	if _, statErr := os.Stat(filepath.Join(target, "carry", "before.txt")); statErr != nil {
		t.Fatalf("entries before the failure were not copied: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(target, "carry", "z-last.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("entries after the failure were copied: %v", statErr)
	}
}
