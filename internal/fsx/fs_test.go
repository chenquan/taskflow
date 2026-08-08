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
