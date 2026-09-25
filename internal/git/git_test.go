package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
