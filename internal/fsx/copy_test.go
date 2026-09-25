package fsx

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeManifest(t *testing.T, source string, lines ...string) *Manifest {
	t.Helper()
	path := filepath.Join(source, ManifestName)
	raw := ""
	for _, line := range lines {
		raw += line + "\n"
	}
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(source)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	return manifest
}

func TestOverlayCopiesMatchedPathsOnly(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(filepath.Join(source, "config", "dev"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "tracked.txt"), []byte("base"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "unlisted.txt"), []byte("unlisted"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "config", "dev", "settings local.env"), []byte("debug=true"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "root.log"), []byte("root log"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "config", "nested.log"), []byte("nested log"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := writeManifest(t, source, "tracked.txt", "config/dev/", "*.log")
	stats, unmatched, err := Overlay(source, target, manifest)
	if err != nil {
		t.Fatalf("Overlay: %v", err)
	}
	if len(unmatched) != 0 {
		t.Fatalf("unexpected unmatched patterns: %v", unmatched)
	}
	for _, required := range []string{"tracked.txt", "root.log", filepath.Join("config", "nested.log"), filepath.Join("config", "dev", "settings local.env")} {
		if _, err := os.Stat(filepath.Join(target, required)); err != nil {
			t.Fatalf("matched entry %s missing: %v", required, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "unlisted.txt")); !os.IsNotExist(err) {
		t.Fatalf("unlisted entry was copied: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(target, "config", "dev", "settings local.env"))
	if err != nil || string(contents) != "debug=true" {
		t.Fatalf("nested file: %q err=%v", contents, err)
	}
	info, err := os.Stat(filepath.Join(target, "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0644 {
		t.Fatalf("file mode=%v", info.Mode().Perm())
	}
	if stats.Entries != 5 || stats.Bytes != int64(len("base")+len("root log")+len("nested log")+len("debug=true")) {
		t.Fatalf("stats=%#v", stats)
	}
}

func TestOverlayCreatesParentDirectoriesForMatchedFiles(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(filepath.Join(source, "a", "b"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "a", "b", "deep.txt"), []byte("deep"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := writeManifest(t, source, "a/b/deep.txt")
	if _, unmatched, err := Overlay(source, target, manifest); err != nil || len(unmatched) != 0 {
		t.Fatalf("Overlay: err=%v unmatched=%v", err, unmatched)
	}
	if _, err := os.Stat(filepath.Join(target, "a", "b", "deep.txt")); err != nil {
		t.Fatalf("deep matched file missing: %v", err)
	}
}

func TestOverlayDirectoryStarMatchesSubtrees(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(filepath.Join(source, "logs", "keep", "deep"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "logs", "keep", "deep", "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "outside.txt"), []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := writeManifest(t, source, "logs/**")
	if _, unmatched, err := Overlay(source, target, manifest); err != nil || len(unmatched) != 0 {
		t.Fatalf("Overlay: err=%v unmatched=%v", err, unmatched)
	}
	if _, err := os.Stat(filepath.Join(target, "logs", "keep", "deep", "a.txt")); err != nil {
		t.Fatalf("** subtree missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "outside.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside entry was copied: %v", err)
	}
}

func TestOverlayExcludesGitEntriesAtRootAndNestedDepth(t *testing.T) {
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
	manifest := writeManifest(t, source, "nested/")
	stats, unmatched, err := Overlay(source, target, manifest)
	if err != nil {
		t.Fatalf("Overlay: %v", err)
	}
	if len(unmatched) != 0 {
		t.Fatalf("unexpected unmatched patterns: %v", unmatched)
	}
	for _, forbidden := range []string{filepath.Join(".git", "HEAD"), filepath.Join("nested", ".git")} {
		if _, err := os.Stat(filepath.Join(target, forbidden)); !os.IsNotExist(err) {
			t.Fatalf(".git entry %s was copied: %v", forbidden, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "nested", "code.txt")); err != nil {
		t.Fatalf("nested working file missing: %v", err)
	}
	if stats.Entries != 2 {
		t.Fatalf("stats=%#v", stats)
	}
}

func TestOverlayRejectsOverlappingRoots(t *testing.T) {
	source := t.TempDir()
	nested := filepath.Join(source, "inner")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	manifest := writeManifest(t, source, "inner/")
	if _, _, err := Overlay(source, nested, manifest); err == nil {
		t.Fatal("target inside source unexpectedly accepted")
	}
	if _, _, err := Overlay(nested, source, manifest); err == nil {
		t.Fatal("source inside target unexpectedly accepted")
	}
	var copyErr *CopyError
	if _, _, err := Overlay(source, nested, manifest); !errors.As(err, &copyErr) || copyErr.Op != "boundary" {
		t.Fatalf("expected boundary error, got %v", err)
	}
}

func TestOverlayRequiresLoadedManifest(t *testing.T) {
	if _, _, err := Overlay(t.TempDir(), filepath.Join(t.TempDir(), "target"), nil); err == nil {
		t.Fatal("nil manifest unexpectedly accepted")
	}
}

func TestOverlayPreservesSymlinksWithoutFollowing(t *testing.T) {
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
	manifest := writeManifest(t, source, "absolute", "dangling")
	if _, unmatched, err := Overlay(source, target, manifest); err != nil || len(unmatched) != 0 {
		t.Fatalf("Overlay: err=%v unmatched=%v", err, unmatched)
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

func TestOverlayReportsUnmatchedLiteralPatterns(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(filepath.Join(source, "real.txt"), []byte("real"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := writeManifest(t, source, "real.txt", "missing.txt", "*.log")
	stats, unmatched, err := Overlay(source, target, manifest)
	if err != nil {
		t.Fatalf("Overlay: %v", err)
	}
	if len(unmatched) != 1 || unmatched[0] != "missing.txt" {
		t.Fatalf("unmatched=%v", unmatched)
	}
	if stats.Entries != 1 {
		t.Fatalf("stats=%#v", stats)
	}
}
