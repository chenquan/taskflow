package fsx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithin(t *testing.T) {
	d := t.TempDir()
	if !Within(d, filepath.Join(d, "a")) {
		t.Fatal("inside")
	}
	if Within(d, filepath.Join(d, "..", "escape")) {
		t.Fatal("escape")
	}
}

func TestRequireWithin(t *testing.T) {
	root := t.TempDir()
	if err := RequireWithin(root, filepath.Join(root, "child")); err != nil {
		t.Fatal(err)
	}
	if err := RequireWithin(root, filepath.Join(root, "..", "escape")); err == nil {
		t.Fatal("expected escape rejection")
	}
}
func TestAtomicWrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a", "b")
	if err := AtomicWrite(p, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "x" {
		t.Fatal(err, string(b))
	}
}

func TestCanonicalManagedResolvesExistingSymlinkParent(t *testing.T) {
	realRoot := t.TempDir()
	linkParent := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(realRoot, linkParent); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	managed, err := CanonicalManaged(filepath.Join(linkParent, "missing", "target"))
	if err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := CanonicalExisting(realRoot)
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(canonicalRoot, "missing", "target")
	if managed != expected {
		t.Fatalf("got %q want %q", managed, expected)
	}
}
