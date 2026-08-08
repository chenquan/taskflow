package lock

import (
	"github.com/gofrs/flock"
	"os"
	"path/filepath"
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
func (l *Lock) Release() error { return l.f.Unlock() }

type ErrConflict struct{}

func (ErrConflict) Error() string { return "task lock is held" }
