package fsx

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeManifestFile(t *testing.T, source, raw string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(source, ManifestName), []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadManifestParsesPatterns(t *testing.T) {
	source := t.TempDir()
	writeManifestFile(t, source, "# comment\n\ntracked.txt\r\nenv/\nconfig/local.yaml\nlogs/**\n")
	manifest, err := LoadManifest(source)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(manifest.Patterns) != 4 {
		t.Fatalf("patterns=%#v", manifest.Patterns)
	}
	file, dir, anchored, doubleStar := manifest.Patterns[0], manifest.Patterns[1], manifest.Patterns[2], manifest.Patterns[3]
	if file.DirOnly || file.Anchored || !file.Literal {
		t.Fatalf("plain file pattern: %#v", file)
	}
	if !dir.DirOnly || dir.Anchored || !dir.Literal {
		t.Fatalf("directory pattern: %#v", dir)
	}
	if anchored.DirOnly || !anchored.Anchored || !anchored.Literal {
		t.Fatalf("anchored pattern: %#v", anchored)
	}
	if doubleStar.Literal {
		t.Fatalf("** pattern must not be literal: %#v", doubleStar)
	}
}

func TestLoadManifestMissingFailsLoudly(t *testing.T) {
	source := t.TempDir()
	_, err := LoadManifest(source)
	var manifestErr *ManifestError
	if !errors.As(err, &manifestErr) || !manifestErr.Missing {
		t.Fatalf("expected missing manifest error, got %v", err)
	}
	if manifestErr.Path != filepath.Join(source, ManifestName) {
		t.Fatalf("error path=%q", manifestErr.Path)
	}
}

func TestLoadManifestRejectsInvalidPatterns(t *testing.T) {
	cases := map[string]string{
		"negation":     "!keep.txt\n",
		"absolute":     "/etc/passwd\n",
		"escape":       "../outside.txt\n",
		"current":      "./here.txt\n",
		"empty":        "/\n",
		"double slash": "a//b\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			source := t.TempDir()
			writeManifestFile(t, source, raw)
			_, err := LoadManifest(source)
			var manifestErr *ManifestError
			if !errors.As(err, &manifestErr) || manifestErr.Missing || manifestErr.Line == 0 {
				t.Fatalf("expected invalid-manifest error with a line number, got %v", err)
			}
		})
	}
}

func TestManifestSelectMatches(t *testing.T) {
	manifest := &Manifest{Patterns: []Pattern{}}
	for _, raw := range []string{"tracked.txt", "env/", "config/local.yaml", "*.log", "logs/**", "gen/*/out.bin"} {
		pattern, err := parsePattern(raw)
		if err != nil {
			t.Fatalf("parsePattern(%q): %v", raw, err)
		}
		manifest.Patterns = append(manifest.Patterns, pattern)
	}
	cases := []struct {
		path   string
		isDir  bool
		expect bool
	}{
		{"tracked.txt", false, true},
		{"deep/nested/tracked.txt", false, true},
		{"env", true, true},
		{"deep/env", true, true},
		{"env/file.txt", false, false},
		{"config/local.yaml", false, true},
		{"config/other.yaml", false, false},
		{"a.log", false, true},
		{"deep/b.log", false, true},
		{"logs", true, true},
		{"logs/x.txt", false, true},
		{"logs/a/b/c.txt", false, true},
		{"logsx", true, false},
		{"gen/a/out.bin", false, true},
		{"gen/a/b/out.bin", false, false},
		{"other/out.bin", false, false},
	}
	for _, tc := range cases {
		if got := manifest.Select(tc.path, tc.isDir, nil); got != tc.expect {
			t.Fatalf("Select(%q, dir=%v)=%v, want %v", tc.path, tc.isDir, got, tc.expect)
		}
	}
}

func TestManifestCommentOnlySelectsNothing(t *testing.T) {
	source := t.TempDir()
	writeManifestFile(t, source, "# nothing to copy\n\n")
	manifest, err := LoadManifest(source)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if manifest.Select("anything.txt", false, nil) {
		t.Fatal("comment-only manifest selected a path")
	}
	if len(manifest.Patterns) != 0 {
		t.Fatalf("patterns=%#v", manifest.Patterns)
	}
}
