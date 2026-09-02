package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/chenquan/taskflow/internal/execx"
)

func TestClientInspectsAndCreatesWorktrees(t *testing.T) {
	root := filepath.Join(t.TempDir(), "source repo")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main", root}, {"-C", root, "config", "user.email", "test@example.com"}, {"-C", root, "config", "user.name", "Test"}, {"-C", root, "commit", "--allow-empty", "-m", "initial"}} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	client := Client{Runner: execx.OSRunner{}}
	info, err := client.Inspect(context.Background(), root)
	if err != nil || info.Root == "" || info.CommonDir == "" || info.Head == "" || info.Branch != "main" || info.Dirty {
		t.Fatalf("err=%v info=%#v", err, info)
	}
	if err := os.WriteFile(filepath.Join(root, "dirty one"), []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dirty-two"), []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dirty three"), []byte("three"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err = client.Inspect(context.Background(), root)
	if err != nil || !info.Dirty || info.DirtyFiles != 3 {
		t.Fatalf("err=%v info=%#v", err, info)
	}
	for _, name := range []string{"dirty one", "dirty-two", "dirty three"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(t.TempDir(), "feature worktree")
	if err := client.AddWorktree(context.Background(), root, "feature/test", target, "main", true); err != nil {
		t.Fatal(err)
	}
	worktrees, err := client.Worktrees(context.Background(), root)
	if err != nil || len(worktrees) != 2 {
		t.Fatalf("err=%v worktrees=%#v", err, worktrees)
	}
	targetInfo, err := client.Inspect(context.Background(), target)
	if err != nil || targetInfo.Branch != "feature/test" || targetInfo.CommonDir != info.CommonDir {
		t.Fatalf("err=%v source=%#v target=%#v", err, info, targetInfo)
	}
	if !client.HasRef(context.Background(), root, "main") || client.HasRef(context.Background(), root, "missing") {
		t.Fatal("unexpected ref inspection")
	}
}

func TestClientDefaultBaseResolvesOriginHead(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	if out, err := exec.Command("git", "init", "-b", "trunk", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	for _, args := range [][]string{{"-C", root, "config", "user.email", "test@example.com"}, {"-C", root, "config", "user.name", "Test"}, {"-C", root, "commit", "--allow-empty", "-m", "init"}, {"-C", root, "remote", "add", "origin", "https://example.test/repo.git"}, {"-C", root, "update-ref", "refs/remotes/origin/trunk", "HEAD"}, {"-C", root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	client := Client{Runner: execx.OSRunner{}}
	base, err := client.DefaultBase(context.Background(), root)
	if err != nil || base != "origin/trunk" {
		t.Fatalf("base=%q err=%v", base, err)
	}
	if out, err := exec.Command("git", "-C", root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/heads/main").CombinedOutput(); err != nil {
		t.Fatalf("set invalid origin/HEAD: %v: %s", err, out)
	}
	if _, err := client.DefaultBase(context.Background(), root); err == nil {
		t.Fatal("expected invalid origin/HEAD error")
	}
	if out, err := exec.Command("git", "-C", root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/missing").CombinedOutput(); err != nil {
		t.Fatalf("set unavailable origin/HEAD: %v: %s", err, out)
	}
	if _, err := client.DefaultBase(context.Background(), root); err == nil {
		t.Fatal("expected unavailable remote default error")
	}
	if out, err := exec.Command("git", "-C", root, "symbolic-ref", "--delete", "refs/remotes/origin/HEAD").CombinedOutput(); err != nil {
		t.Fatalf("remove origin/HEAD: %v: %s", err, out)
	}
	if _, err := client.DefaultBase(context.Background(), root); err == nil {
		t.Fatal("expected missing origin/HEAD error")
	}
}

func TestClientAddWorktreeDefaultDoesNotTrackRemoteBase(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	if out, err := exec.Command("git", "init", "-b", "master", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	for _, args := range [][]string{
		{"-C", root, "config", "user.email", "test@example.com"},
		{"-C", root, "config", "user.name", "Test"},
		{"-C", root, "commit", "--allow-empty", "-m", "init"},
		{"-C", root, "remote", "add", "origin", "https://example.test/repo.git"},
		{"-C", root, "update-ref", "refs/remotes/origin/master", "HEAD"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	target := filepath.Join(t.TempDir(), "worktree")
	client := Client{Runner: execx.OSRunner{}}
	if err := client.AddWorktree(context.Background(), root, "feature/master-base", target, "origin/master", false); err != nil {
		t.Fatal(err)
	}
	info, err := client.Inspect(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Upstream != "" {
		t.Fatalf("upstream=%q, want no upstream", info.Upstream)
	}
	if info.Head == "" || !client.HasRef(context.Background(), root, "origin/master") {
		t.Fatalf("worktree did not start from origin/master: info=%#v", info)
	}

	trackedTarget := filepath.Join(t.TempDir(), "tracked-worktree")
	if err := client.AddWorktree(context.Background(), root, "feature/explicit-master", trackedTarget, "origin/master", true); err != nil {
		t.Fatal(err)
	}
	trackedInfo, err := client.Inspect(context.Background(), trackedTarget)
	if err != nil {
		t.Fatal(err)
	}
	if trackedInfo.Upstream != "origin/master" {
		t.Fatalf("explicit upstream=%q, want origin/master", trackedInfo.Upstream)
	}
}

func TestClientLocalPathQueriesPreserveNULDelimitedNames(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repo")
	if out, err := exec.Command("git", "init", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	for _, args := range [][]string{{"-C", root, "config", "user.email", "test@example.com"}, {"-C", root, "config", "user.name", "Test"}, {"-C", root, "commit", "--allow-empty", "-m", "init"}} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ignored"), 0755); err != nil {
		t.Fatal(err)
	}
	normalName := "normal\nname"
	if runtime.GOOS == "windows" {
		normalName = "normal name"
	}
	normal := filepath.Join(root, normalName)
	ignored := filepath.Join(root, "ignored", "ignored name")
	if err := os.WriteFile(normal, []byte("normal"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignored, []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", root, "add", ".gitignore").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", root, "commit", "-m", "ignore").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
	client := Client{Runner: execx.OSRunner{}}
	untracked, err := client.UntrackedPaths(context.Background(), root, []string{"."})
	if err != nil || len(untracked) != 1 || untracked[0] != normalName {
		t.Fatalf("untracked=%#v err=%v", untracked, err)
	}
	ignoredPaths, err := client.IgnoredPaths(context.Background(), root, []string{"ignored"})
	if err != nil || len(ignoredPaths) != 1 || ignoredPaths[0] != "ignored/ignored name" {
		t.Fatalf("ignored=%#v err=%v", ignoredPaths, err)
	}
	tracked, err := client.TrackedPaths(context.Background(), root, []string{".gitignore"})
	if err != nil || len(tracked) != 1 || tracked[0] != ".gitignore" {
		t.Fatalf("tracked=%#v err=%v", tracked, err)
	}
	base, err := client.BasePaths(context.Background(), root, "HEAD")
	if err != nil || len(base) != 1 || base[0] != ".gitignore" {
		t.Fatalf("base=%#v err=%v", base, err)
	}
}
