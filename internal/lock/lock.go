package lock

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

type Lock struct{ f *flock.Flock }

func Acquire(taskRoot string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Join(taskRoot, ".specflow"), 0755); err != nil {
		return nil, err
	}
	f := flock.New(filepath.Join(taskRoot, ".specflow", "lock"))
	ok, err := f.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrConflict{}
	}
	return &Lock{f}, nil
}

func AcquireSource(commonDir, branch string) (*Lock, error) {
	if commonDir == "" || branch == "" {
		return nil, fmt.Errorf("source lock requires common Git directory and branch")
	}
	digest := sha256.Sum256([]byte(commonDir + "\x00" + branch))
	directory := filepath.Join(commonDir, "specflow-locks")
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}
	f := flock.New(filepath.Join(directory, fmt.Sprintf("%x.lock", digest)))
	ok, err := f.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrConflict{}
	}
	return &Lock{f}, nil
}
func (l *Lock) Release() error { return l.f.Unlock() }

type ErrConflict struct{}

func (ErrConflict) Error() string { return "task lock is held" }
