package fsx

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyTreeCopiesCompleteTree(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(filepath.Join(source, "config", "dev"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "tracked.txt"), []byte("base"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "config", "dev", "settings local.env"), []byte("debug=true"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "empty"), 0750); err != nil {
		t.Fatal(err)
	}
	stats, err := CopyTree(source, target)
	if err != nil {
		t.Fatalf("CopyTree: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(target, "config", "dev", "settings local.env"))
	if err != nil || string(contents) != "debug=true" {
		t.Fatalf("nested file: %q err=%v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(target, "empty")); err != nil {
		t.Fatalf("empty directory missing: %v", err)
	}
	info, err := os.Stat(filepath.Join(target, "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0644 {
		t.Fatalf("file mode=%v", info.Mode().Perm())
	}
	if stats.Entries != 5 || stats.Bytes != int64(len("base")+len("debug=true")) {
		t.Fatalf("stats=%#v", stats)
	}
}

func TestCopyTreeExcludesGitEntriesAtRootAndNestedDepth(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", ".git"), []byte("gitdir: elsewhere"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "code.txt"), []byte("code"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "root.env"), []byte("root"), 0644); err != nil {
		t.Fatal(err)
	}
	stats, err := CopyTree(source, target)
	if err != nil {
		t.Fatalf("CopyTree: %v", err)
	}
	for _, forbidden := range []string{filepath.Join(".git", "HEAD"), filepath.Join("nested", ".git")} {
		if _, err := os.Stat(filepath.Join(target, forbidden)); !os.IsNotExist(err) {
			t.Fatalf(".git entry %s was copied: %v", forbidden, err)
		}
	}
	for _, required := range []string{"root.env", filepath.Join("nested", "code.txt")} {
		if _, err := os.Stat(filepath.Join(target, required)); err != nil {
			t.Fatalf("entry %s missing: %v", required, err)
		}
	}
	if stats.Entries != 3 {
		t.Fatalf("stats=%#v", stats)
	}
}

func TestCopyTreeRejectsOverlappingRoots(t *testing.T) {
	source := t.TempDir()
	nested := filepath.Join(source, "inner")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := CopyTree(source, nested); err == nil {
		t.Fatal("target inside source unexpectedly accepted")
	}
	if _, err := CopyTree(nested, source); err == nil {
		t.Fatal("source inside target unexpectedly accepted")
	}
	var copyErr *CopyError
	_, err := CopyTree(source, nested)
	if !errors.As(err, &copyErr) || copyErr.Op != "boundary" {
		t.Fatalf("expected boundary error, got %v", err)
	}
}

func TestCopyTreePreservesSymlinksWithoutFollowing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on windows")
	}
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "absolute")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("nowhere", "dangling.txt"), filepath.Join(source, "dangling")); err != nil {
		t.Fatal(err)
	}
	if _, err := CopyTree(source, target); err != nil {
		t.Fatalf("CopyTree: %v", err)
	}
	resolved, err := os.Readlink(filepath.Join(target, "absolute"))
	if err != nil || resolved != outside {
		t.Fatalf("absolute link=%q err=%v", resolved, err)
	}
	resolved, err = os.Readlink(filepath.Join(target, "dangling"))
	if err != nil || filepath.IsAbs(resolved) {
		t.Fatalf("dangling link=%q err=%v", resolved, err)
	}
}
