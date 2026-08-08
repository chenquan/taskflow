package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireRelease(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specflow"), 0755); err != nil {
		t.Fatal(err)
	}
	h, e := Acquire(root, "codex", "/tmp/primary")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = Acquire(root, "claude", "/tmp/primary"); e == nil {
		t.Fatal("expected conflict")
	}
	if e = h.Release(); e != nil {
		t.Fatal(e)
	}
}
func TestActive(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specflow"), 0755); err != nil {
		t.Fatal(err)
	}
	h, e := Acquire(root, "codex", "/tmp/primary")
	if e != nil {
		t.Fatal(e)
	}
	defer h.Release()
	lease, e := Active(root)
	if e != nil || lease == nil || lease.Tool != "codex" {
		t.Fatalf("%v %#v", e, lease)
	}
}
