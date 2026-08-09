package lock

import (
	"path/filepath"
	"testing"
)

func TestAcquireConflictAndRelease(t *testing.T) {
	root := t.TempDir()
	first, err := Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(root); err == nil {
		t.Fatal("expected lock conflict")
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireSourceCoordinatesBranchesIndependently(t *testing.T) {
	commonDir := filepath.Join(t.TempDir(), "source.git")
	first, err := AcquireSource(commonDir, "feature/task")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	if _, err := AcquireSource(commonDir, "feature/task"); err == nil {
		t.Fatal("expected same source branch conflict")
	}
	other, err := AcquireSource(commonDir, "feature/other")
	if err != nil {
		t.Fatalf("different branch should not conflict: %v", err)
	}
	if err := other.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireSourceRequiresIdentity(t *testing.T) {
	for _, values := range [][2]string{{"", "branch"}, {"common", ""}} {
		if _, err := AcquireSource(values[0], values[1]); err == nil {
			t.Fatalf("expected invalid source identity for %#v", values)
		}
	}
}

func TestConflictError(t *testing.T) {
	if (ErrConflict{}).Error() == "" {
		t.Fatal("empty conflict error")
	}
}
