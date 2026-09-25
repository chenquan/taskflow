package fsx

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyStats reports how many filesystem entries were copied and how many
// regular-file bytes were written.
type CopyStats struct {
	Entries int64
	Bytes   int64
}

// CopyError identifies the failing copy operation and the path relative to the
// copied root so diagnostics stay stable across platforms.
type CopyError struct {
	Op   string // boundary, manifest, lstat, mkdir, readlink, symlink, open, create, read, write, chmod, unsupported-entry
	Path string // relative to the copied root; "." denotes the roots themselves
	Err  error
}

func (e *CopyError) Error() string {
	if e.Path == "" || e.Path == "." {
		return fmt.Sprintf("copy %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("copy %s %s: %v", e.Op, e.Path, e.Err)
}

func (e *CopyError) Unwrap() error { return e.Err }

// Overlay copies the manifest-matched source paths into target. The walk
// carries a matched directory's whole subtree, creates missing parent
// directories of carried entries, and reports literal patterns that matched no
// source path. Every `.git` entry — the root entry and any entry at a nested
// depth — is excluded so the target keeps only its own Git metadata. Regular
// file and directory permission bits are preserved, symlinks are reproduced
// without following them, and any other carried entry type fails the copy.
// Source and target must not contain one another.
func Overlay(source, target string, manifest *Manifest) (CopyStats, []string, error) {
	if manifest == nil {
		return CopyStats{}, nil, &CopyError{Op: "manifest", Err: fmt.Errorf("no copy manifest was loaded")}
	}
	if Within(source, target) || Within(target, source) {
		return CopyStats{}, nil, &CopyError{Op: "boundary", Err: fmt.Errorf("source %q and target %q must not contain one another", source, target)}
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return CopyStats{}, nil, &CopyError{Op: "mkdir", Path: ".", Err: err}
	}
	var stats CopyStats
	literalHits := make([]bool, len(manifest.Patterns))
	// carried marks directories whose subtree is copied wholesale because the
	// directory itself or an ancestor matched.
	carried := map[string]bool{}
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return &CopyError{Op: "lstat", Path: relativePath(source, path), Err: walkErr}
		}
		relative := relativePath(source, path)
		if entry.Name() == ".git" {
			// The target keeps its own Git metadata; a nested entry belongs to
			// another checkout and must never leak into the snapshot.
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if relative == "." {
			return nil
		}
		isDir := entry.IsDir()
		directlyMatched := manifest.Select(filepath.ToSlash(relative), isDir, literalHits)
		if !directlyMatched && !carried[filepath.Dir(relative)] {
			return nil
		}
		// Record ancestor-carried directories too, so deeper descendants see
		// the wholesale carry without re-deriving the ancestor chain.
		if isDir {
			carried[relative] = true
		}
		info, err := entry.Info()
		if err != nil {
			return &CopyError{Op: "lstat", Path: relative, Err: err}
		}
		destination := filepath.Join(target, filepath.FromSlash(relative))
		switch {
		case isDir:
			if err := os.MkdirAll(destination, info.Mode().Perm()); err != nil {
				return &CopyError{Op: "mkdir", Path: relative, Err: err}
			}
			stats.Entries++
		case info.Mode()&fs.ModeSymlink != 0:
			if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
				return &CopyError{Op: "mkdir", Path: relative, Err: err}
			}
			if err := copySymlink(source, target, relative); err != nil {
				return err
			}
			stats.Entries++
		case info.Mode().IsRegular():
			if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
				return &CopyError{Op: "mkdir", Path: relative, Err: err}
			}
			written, err := copyRegularFile(path, destination, info.Mode().Perm())
			if err != nil {
				if copyErr, ok := err.(*CopyError); ok {
					if copyErr.Path == "" || copyErr.Path == "." {
						copyErr.Path = relative
					}
					return copyErr
				}
				return &CopyError{Op: "write", Path: relative, Err: err}
			}
			stats.Entries++
			stats.Bytes += written
		default:
			return &CopyError{Op: "unsupported-entry", Path: relative, Err: fmt.Errorf("unsupported entry type %s", info.Mode().Type())}
		}
		return nil
	})
	if err != nil {
		return stats, nil, err
	}
	var unmatched []string
	for index, pattern := range manifest.Patterns {
		if pattern.Literal && !literalHits[index] {
			unmatched = append(unmatched, pattern.Raw)
		}
	}
	return stats, unmatched, nil
}

func copySymlink(source, target, relative string) error {
	sourcePath := filepath.Join(source, filepath.FromSlash(relative))
	destination := filepath.Join(target, filepath.FromSlash(relative))
	linkTarget, err := os.Readlink(sourcePath)
	if err != nil {
		return &CopyError{Op: "readlink", Path: relative, Err: err}
	}
	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return &CopyError{Op: "symlink", Path: relative, Err: err}
	}
	if err := os.Symlink(linkTarget, destination); err != nil {
		return &CopyError{Op: "symlink", Path: relative, Err: err}
	}
	return nil
}

func copyRegularFile(source, target string, perm os.FileMode) (int64, error) {
	in, err := os.Open(source)
	if err != nil {
		return 0, &CopyError{Op: "open", Err: err}
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return 0, &CopyError{Op: "create", Err: err}
	}
	written, copyErr := io.Copy(out, in)
	if chmodErr := out.Chmod(perm); chmodErr != nil && copyErr == nil {
		copyErr = &CopyError{Op: "chmod", Err: chmodErr}
	}
	if closeErr := out.Close(); closeErr != nil && copyErr == nil {
		copyErr = &CopyError{Op: "write", Err: closeErr}
	}
	if copyErr != nil {
		return written, copyErr
	}
	return written, nil
}

func relativePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return relative
}
