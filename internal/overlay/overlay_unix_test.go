//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package overlay

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/chenquan/taskflow/internal/execx"
	"github.com/chenquan/taskflow/internal/git"
)

func TestDiscoverRejectsFIFO(t *testing.T) {
	root := overlayGitRepo(t)
	fifo := filepath.Join(root, "local.fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Skipf("FIFO unavailable: %v", err)
	}
	defer os.Remove(fifo)
	client := git.Client{Runner: execx.OSRunner{}}
	if _, err := Discover(context.Background(), client, root, []string{"local.fifo"}); overlayErrorCode(err) != CodePathUnsafe {
		t.Fatalf("FIFO error=%v", err)
	}
}
