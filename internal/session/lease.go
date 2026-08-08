package session

import (
	"encoding/json"
	"fmt"
	"github.com/chenquan/specflow/internal/fsx"
	"github.com/gofrs/flock"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type Lease struct {
	PID       int       `json:"pid"`
	Tool      string    `json:"tool"`
	StartedAt time.Time `json:"startedAt"`
	Primary   string    `json:"primary"`
	Token     string    `json:"token"`
}
type Holder struct {
	lease Lease
	lock  *flock.Flock
	path  string
}

func Acquire(root, tool, primary string) (*Holder, error) {
	lk := flock.New(filepath.Join(root, ".specflow", "session.lock"))
	ok, e := lk.TryLock()
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, fmt.Errorf("session lock is held")
	}
	p := filepath.Join(root, ".specflow", "session.json")
	if b, e := os.ReadFile(p); e == nil {
		var old Lease
		if json.Unmarshal(b, &old) == nil && processAlive(old.PID) {
			lk.Unlock()
			return nil, fmt.Errorf("active %s session held by pid %d", old.Tool, old.PID)
		}
	}
	l := Lease{PID: os.Getpid(), Tool: tool, StartedAt: time.Now().UTC(), Primary: primary, Token: fmt.Sprintf("%d", time.Now().UnixNano())}
	b, _ := json.MarshalIndent(l, "", "  ")
	if e = fsx.AtomicWrite(p, append(b, '\n'), 0600); e != nil {
		lk.Unlock()
		return nil, e
	}
	return &Holder{l, lk, p}, nil
}
func (h *Holder) Release() error { _ = os.Remove(h.path); return h.lock.Unlock() }
func Active(root string) (*Lease, error) {
	b, err := os.ReadFile(filepath.Join(root, ".specflow", "session.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var lease Lease
	if err := json.Unmarshal(b, &lease); err != nil {
		return nil, err
	}
	if !processAlive(lease.PID) {
		return nil, nil
	}
	return &lease, nil
}
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, e := os.FindProcess(pid)
	if e != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}
