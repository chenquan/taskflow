package fsx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func CanonicalExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}
func CanonicalManaged(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(abs)
	missing := []string{}
	for {
		if _, statErr := os.Lstat(parent); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			return "", statErr
		}
		missing = append([]string{filepath.Base(parent)}, missing...)
		next := filepath.Dir(parent)
		if next == parent {
			return "", fmt.Errorf("no existing parent for %q", abs)
		}
		parent = next
	}
	p, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{p}, append(missing, filepath.Base(abs))...)...), nil
}
func Within(root, target string) bool {
	r, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	t, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(r, t)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func AtomicWrite(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".specflow-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func RequireWithin(root, target string) error {
	if !Within(root, target) {
		return fmt.Errorf("path %q escapes %q", target, root)
	}
	return nil
}
