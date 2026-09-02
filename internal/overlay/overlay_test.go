package overlay

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/execx"
	"github.com/chenquan/taskflow/internal/git"
)

func TestErrorFormattingAndUnwrap(t *testing.T) {
	cause := errors.New("cause")
	for _, test := range []struct {
		err  *Error
		want string
	}{
		{err: &Error{Code: "CODE"}, want: "CODE"},
		{err: &Error{Code: "CODE", Path: "path"}, want: "CODE: path"},
		{err: &Error{Code: "CODE", Err: cause}, want: "CODE: cause"},
		{err: &Error{Code: "CODE", Path: "path", Err: cause}, want: "CODE path: cause"},
	} {
		if got := test.err.Error(); got != test.want {
			t.Fatalf("error=%q want=%q", got, test.want)
		}
	}
	wrapped := &Error{Code: "CODE", Err: cause}
	if !errors.Is(wrapped, cause) {
		t.Fatal("overlay error does not unwrap its cause")
	}
	if (&Error{Code: "CODE"}).Unwrap() != nil {
		t.Fatal("nil cause unexpectedly unwrapped")
	}
}

func TestNormalizePathsRejectsUnsafeForms(t *testing.T) {
	for _, paths := range [][]string{
		{""},
		{"/absolute/path"},
		{"C:/absolute/path"},
		{".."},
		{"../outside"},
		{".git/config"},
		{"safe", "safe"},
	} {
		if _, err := normalizePaths(paths); overlayErrorCode(err) != CodePathUnsafe {
			t.Fatalf("paths %#v error=%v", paths, err)
		}
	}
	paths, err := normalizePaths([]string{`config\dev`})
	if err != nil || len(paths) != 1 || paths[0] != "config/dev" {
		t.Fatalf("portable paths=%#v err=%v", paths, err)
	}
}

func overlayGitRepo(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "source repo")
	for _, args := range [][]string{
		{"init", "-b", "main", root},
		{"-C", root, "config", "user.email", "test@example.com"},
		{"-C", root, "config", "user.name", "Test"},
		{"-C", root, "commit", "--allow-empty", "-m", "init"},
	} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	return root
}

func overlayErrorCode(err error) string {
	if err != nil && strings.Contains(err.Error(), CodeTrackedFile) {
		return CodeTrackedFile
	}
	if err != nil && strings.Contains(err.Error(), CodeDestinationConflict) {
		return CodeDestinationConflict
	}
	if err != nil && strings.Contains(err.Error(), CodeSourceChanged) {
		return CodeSourceChanged
	}
	if err != nil && strings.Contains(err.Error(), CodeBaseConflict) {
		return CodeBaseConflict
	}
	if err != nil && strings.Contains(err.Error(), CodePathUnsafe) {
		return CodePathUnsafe
	}
	return ""
}

func TestDiscoverUsesGitClassificationAndPreservesSpecialPaths(t *testing.T) {
	root := overlayGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.env\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("tracked"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-C", root, "add", ".gitignore", "tracked.txt"}, {"-C", root, "commit", "-m", "tracked"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "plain file"), []byte("plain"), 0751); err != nil {
		t.Fatal(err)
	}
	plainInfo, err := os.Stat(filepath.Join(root, "plain file"))
	if err != nil {
		t.Fatal(err)
	}
	nestedName := "child\nname"
	if runtime.GOOS == "windows" {
		nestedName = "child name"
	}
	nested := filepath.Join(root, "nested", "dir", nestedName)
	if err := os.MkdirAll(filepath.Dir(nested), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("nested"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.env"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	client := git.Client{Runner: execx.OSRunner{}}
	snapshot, err := Discover(context.Background(), client, root, []string{"plain file", "nested", "ignored.env"})
	if err != nil {
		t.Fatal(err)
	}
	expectedNestedPath := filepath.ToSlash(filepath.Join("nested", "dir", nestedName))
	if len(snapshot.Files) != 3 || snapshot.Files[0].Path != "ignored.env" || snapshot.Files[1].Path != expectedNestedPath || snapshot.Files[2].Path != "plain file" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if snapshot.Files[0].SHA256 == "" || snapshot.Files[1].Size != int64(len("nested")) || snapshot.TotalBytes != int64(len("plain")+len("nested")+len("secret")) {
		t.Fatalf("unexpected file metadata: %#v", snapshot)
	}
	if snapshot.Files[2].Mode != uint32(plainInfo.Mode().Perm()) {
		t.Fatalf("plain mode=%o, want %o", snapshot.Files[2].Mode, plainInfo.Mode().Perm())
	}
	if _, err := Discover(context.Background(), client, root, []string{"tracked.txt"}); overlayErrorCode(err) != CodeTrackedFile {
		t.Fatalf("tracked error=%v", err)
	}
}

func TestMaterializeIsAtomicAndDoesNotOverwrite(t *testing.T) {
	root := overlayGitRepo(t)
	path := filepath.Join(root, "local.env")
	if err := os.WriteFile(path, []byte("source"), 0750); err != nil {
		t.Fatal(err)
	}
	client := git.Client{Runner: execx.OSRunner{}}
	snapshot, err := Discover(context.Background(), client, root, []string{"local.env"})
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	result, err := Materialize(context.Background(), root, target, snapshot)
	if err != nil || result.Copied != 1 || result.Existing != 0 {
		t.Fatalf("materialize result=%#v err=%v", result, err)
	}
	contents, err := os.ReadFile(filepath.Join(target, "local.env"))
	if err != nil || string(contents) != "source" {
		t.Fatalf("copied contents=%q err=%v", contents, err)
	}
	info, err := os.Stat(filepath.Join(target, "local.env"))
	if err != nil || info.Mode().Perm() != os.FileMode(snapshot.Files[0].Mode) {
		t.Fatalf("copied mode=%o err=%v", info.Mode().Perm(), err)
	}
	if err := os.WriteFile(filepath.Join(target, "local.env"), []byte("user"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Materialize(context.Background(), root, target, snapshot); overlayErrorCode(err) != CodeDestinationConflict {
		t.Fatalf("overwrite error=%v", err)
	}
	contents, err = os.ReadFile(filepath.Join(target, "local.env"))
	if err != nil || string(contents) != "user" {
		t.Fatalf("destination changed: %q err=%v", contents, err)
	}
}

func TestValidateBaseCollisionsChecksAncestorsAndDescendants(t *testing.T) {
	snapshot := Snapshot{Files: []domain.OverlayFile{{Path: "config/dev.env"}, {Path: "settings"}}}
	if err := ValidateBaseCollisions(snapshot, []string{"config", "other/file"}); err == nil || overlayErrorCode(err) != CodeBaseConflict {
		t.Fatalf("ancestor collision=%v", err)
	}
	if err := ValidateBaseCollisions(Snapshot{Files: []domain.OverlayFile{{Path: "config"}}}, []string{"config/dev.env"}); err == nil || overlayErrorCode(err) != CodeBaseConflict {
		t.Fatalf("descendant collision=%v", err)
	}
}

func TestDiscoverRejectsSymlinksAndEscapingPaths(t *testing.T) {
	root := overlayGitRepo(t)
	safe := filepath.Join(root, "safe.env")
	if err := os.WriteFile(safe, []byte("safe"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.env")
	if err := os.Symlink(safe, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	client := git.Client{Runner: execx.OSRunner{}}
	if _, err := Discover(context.Background(), client, root, []string{"link.env"}); overlayErrorCode(err) != CodePathUnsafe {
		t.Fatalf("symlink error=%v", err)
	}
	if _, err := Discover(context.Background(), client, root, []string{"../outside"}); overlayErrorCode(err) != CodePathUnsafe {
		t.Fatalf("escape error=%v", err)
	}
	if _, err := Discover(context.Background(), client, root, []string{"missing.env"}); err == nil || !strings.Contains(err.Error(), CodePathNotFound) {
		t.Fatalf("missing error=%v", err)
	}
}

func TestMaterializeRejectsChangedSource(t *testing.T) {
	root := overlayGitRepo(t)
	path := filepath.Join(root, "local.env")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	client := git.Client{Runner: execx.OSRunner{}}
	snapshot, err := Discover(context.Background(), client, root, []string{"local.env"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("after!"), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Materialize(context.Background(), root, target, snapshot); overlayErrorCode(err) != CodeSourceChanged {
		t.Fatalf("changed source error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "local.env")); !os.IsNotExist(err) {
		t.Fatalf("changed source was published: %v", err)
	}
}

func TestVerifySourceAndHasExistingFiles(t *testing.T) {
	source := t.TempDir()
	path := filepath.Join(source, "local.env")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	hash, size, err := hashRegularFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{Files: []domain.OverlayFile{{Path: "local.env", Size: size, Mode: uint32(info.Mode().Perm()), SHA256: hash}}}
	if err := VerifySource(source, snapshot); err != nil {
		t.Fatalf("matching source: %v", err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifySource(source, snapshot); overlayErrorCode(err) != CodeSourceChanged {
		t.Fatalf("changed source: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := VerifySource(source, snapshot); overlayErrorCode(err) != CodeSourceChanged {
		t.Fatalf("missing source: %v", err)
	}

	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	nested := Snapshot{Files: []domain.OverlayFile{{Path: "nested/local.env"}}}
	has, err := HasExistingFiles(target, nested)
	if err != nil || has {
		t.Fatalf("missing destination: has=%v err=%v", has, err)
	}
	if err := os.MkdirAll(filepath.Join(target, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	has, err = HasExistingFiles(target, nested)
	if err != nil || has {
		t.Fatalf("missing nested destination: has=%v err=%v", has, err)
	}
	if err := os.WriteFile(filepath.Join(target, "nested", "local.env"), []byte("present"), 0644); err != nil {
		t.Fatal(err)
	}
	has, err = HasExistingFiles(target, nested)
	if err != nil || !has {
		t.Fatalf("existing destination: has=%v err=%v", has, err)
	}
	if _, err := HasExistingFiles(filepath.Join(target, "missing"), nested); overlayErrorCode(err) != CodeDestinationConflict {
		t.Fatalf("missing target error=%v", err)
	}
	notDirectory := filepath.Join(target, "not-directory")
	if err := os.WriteFile(notDirectory, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := HasExistingFiles(notDirectory, nested); overlayErrorCode(err) != CodeDestinationConflict {
		t.Fatalf("file target error=%v", err)
	}
}

func TestCopyAndPublishHandlesMissingSourceAndExistingMatchingDestination(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	missing := domain.OverlayFile{Path: "missing.env", Size: 1, Mode: 0600, SHA256: strings.Repeat("0", 64)}
	if err := copyAndPublish(context.Background(), source, target, missing); overlayErrorCode(err) != CodeSourceChanged {
		t.Fatalf("missing source error=%v", err)
	}
	path := filepath.Join(source, "local.env")
	if err := os.WriteFile(path, []byte("present"), 0600); err != nil {
		t.Fatal(err)
	}
	hash, size, err := hashRegularFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := domain.OverlayFile{Path: "local.env", Size: size, Mode: uint32(info.Mode().Perm()), SHA256: hash}
	if err := os.WriteFile(filepath.Join(target, expected.Path), []byte("present"), os.FileMode(expected.Mode)); err != nil {
		t.Fatal(err)
	}
	if err := copyAndPublish(context.Background(), source, target, expected); err != nil {
		t.Fatalf("matching existing destination: %v", err)
	}
}
