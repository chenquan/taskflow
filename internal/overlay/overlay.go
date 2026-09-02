package overlay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chenquan/taskflow/internal/domain"
	"github.com/chenquan/taskflow/internal/fsx"
	"github.com/chenquan/taskflow/internal/git"
)

const (
	CodePathNotFound        = "OVERLAY_PATH_NOT_FOUND"
	CodePathUnsafe          = "OVERLAY_PATH_UNSAFE"
	CodeTrackedFile         = "OVERLAY_TRACKED_FILE"
	CodeDiscoveryFailed     = "OVERLAY_DISCOVERY_FAILED"
	CodeBaseConflict        = "OVERLAY_BASE_CONFLICT"
	CodeSourceChanged       = "OVERLAY_SOURCE_CHANGED"
	CodeDestinationConflict = "OVERLAY_DESTINATION_CONFLICT"
	CodeCopyFailed          = "OVERLAY_COPY_FAILED"
)

// Error is a stable overlay diagnostic that the app layer can expose through
// the CLI result envelope without parsing filesystem error strings.
type Error struct {
	Code string
	Path string
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		if e.Path == "" {
			return e.Code
		}
		return fmt.Sprintf("%s: %s", e.Code, e.Path)
	}
	if e.Path == "" {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("%s %s: %v", e.Code, e.Path, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// Snapshot is a deterministic creation-time view of the selected local
// files. Paths preserves the normalized configuration order; Files is sorted
// by source-relative path for stable reports and ownership metadata.
type Snapshot struct {
	Paths      []string
	Files      []domain.OverlayFile
	TotalBytes int64
}

type DestinationCheck struct {
	Path  string
	State string // missing or match
}

type MaterializeResult struct {
	Copied   int
	Existing int
}

// Discover resolves explicit source-relative paths and asks Git which files
// are untracked or ignored. The filesystem walk is intentionally used only to
// validate types and gather metadata; Git remains the source of truth for
// tracked/ignored classification.
func Discover(ctx context.Context, client git.Client, source string, configured []string) (Snapshot, error) {
	paths, err := normalizePaths(configured)
	if err != nil {
		return Snapshot{}, err
	}
	files := map[string]domain.OverlayFile{}
	for _, relative := range paths {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		absolute := filepath.Join(source, filepath.FromSlash(relative))
		info, err := os.Lstat(absolute)
		if err != nil {
			if os.IsNotExist(err) {
				return Snapshot{}, &Error{Code: CodePathNotFound, Path: relative, Err: err}
			}
			return Snapshot{}, &Error{Code: CodeDiscoveryFailed, Path: relative, Err: err}
		}
		if err := validateEntryType(relative, info); err != nil {
			return Snapshot{}, err
		}

		candidates, err := filesystemCandidates(source, absolute, relative, info)
		if err != nil {
			return Snapshot{}, err
		}
		selected, err := client.SelectedLocalPaths(ctx, source, []string{relative})
		if err != nil {
			return Snapshot{}, &Error{Code: CodeDiscoveryFailed, Path: relative, Err: err}
		}
		selectedSet := make(map[string]struct{}, len(selected))
		for _, candidate := range selected {
			selectedSet[normalizeGitPath(candidate)] = struct{}{}
		}
		if !info.IsDir() {
			tracked, err := client.TrackedPaths(ctx, source, []string{relative})
			if err != nil {
				return Snapshot{}, &Error{Code: CodeDiscoveryFailed, Path: relative, Err: err}
			}
			for _, trackedPath := range tracked {
				if normalizeGitPath(trackedPath) == relative {
					return Snapshot{}, &Error{Code: CodeTrackedFile, Path: relative, Err: fmt.Errorf("explicitly selected file is tracked")}
				}
			}
		}
		for candidate, candidateInfo := range candidates {
			if _, ok := selectedSet[candidate]; !ok {
				continue
			}
			if _, ok := files[candidate]; ok {
				continue
			}
			file, err := snapshotFile(filepath.Join(source, filepath.FromSlash(candidate)), candidate, candidateInfo)
			if err != nil {
				return Snapshot{}, err
			}
			files[candidate] = file
		}
	}

	result := Snapshot{Paths: append([]string(nil), paths...), Files: make([]domain.OverlayFile, 0, len(files))}
	for _, file := range files {
		result.Files = append(result.Files, file)
		result.TotalBytes += file.Size
	}
	sort.Slice(result.Files, func(left, right int) bool { return result.Files[left].Path < result.Files[right].Path })
	return result, nil
}

func normalizePaths(configured []string) ([]string, error) {
	paths := make([]string, 0, len(configured))
	seen := map[string]struct{}{}
	for _, raw := range configured {
		if strings.TrimSpace(raw) == "" {
			return nil, &Error{Code: CodePathUnsafe, Path: raw, Err: fmt.Errorf("overlay path must not be empty")}
		}
		portable := strings.ReplaceAll(raw, `\`, "/")
		if filepath.IsAbs(raw) || strings.HasPrefix(portable, "/") || isWindowsAbsolute(portable) {
			return nil, &Error{Code: CodePathUnsafe, Path: raw, Err: fmt.Errorf("overlay path must be source-relative")}
		}
		clean := pathpkg.Clean(portable)
		if clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, &Error{Code: CodePathUnsafe, Path: raw, Err: fmt.Errorf("overlay path escapes the source")}
		}
		if hasGitComponent(clean) {
			return nil, &Error{Code: CodePathUnsafe, Path: raw, Err: fmt.Errorf("overlay path may not contain .git")}
		}
		if _, ok := seen[clean]; ok {
			return nil, &Error{Code: CodePathUnsafe, Path: clean, Err: fmt.Errorf("duplicate overlay path")}
		}
		seen[clean] = struct{}{}
		paths = append(paths, clean)
	}
	return paths, nil
}

func filesystemCandidates(source, absolute, relative string, info os.FileInfo) (map[string]os.FileInfo, error) {
	candidates := map[string]os.FileInfo{}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return nil, &Error{Code: CodePathUnsafe, Path: relative, Err: fmt.Errorf("only regular files and directories are supported")}
		}
		candidates[relative] = info
		return candidates, nil
	}
	err := filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return &Error{Code: CodeDiscoveryFailed, Path: relative, Err: walkErr}
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return &Error{Code: CodeDiscoveryFailed, Path: relative, Err: err}
		}
		rel = normalizeGitPath(rel)
		if rel != "." && hasGitComponent(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return &Error{Code: CodePathUnsafe, Path: rel, Err: fmt.Errorf("only regular files are supported")}
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return &Error{Code: CodeDiscoveryFailed, Path: rel, Err: err}
		}
		candidates[rel] = fileInfo
		return nil
	})
	return candidates, err
}

func validateEntryType(relative string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return &Error{Code: CodePathUnsafe, Path: relative, Err: fmt.Errorf("only regular files and directories are supported")}
	}
	return nil
}

func snapshotFile(path, relative string, expected os.FileInfo) (domain.OverlayFile, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return domain.OverlayFile{}, &Error{Code: CodeSourceChanged, Path: relative, Err: err}
	}
	if err := validateEntryType(relative, before); err != nil {
		return domain.OverlayFile{}, err
	}
	if expected != nil && (before.Size() != expected.Size() || before.Mode().Perm() != expected.Mode().Perm()) {
		return domain.OverlayFile{}, &Error{Code: CodeSourceChanged, Path: relative, Err: fmt.Errorf("file metadata changed during discovery")}
	}
	hash, size, err := hashRegularFile(path)
	if err != nil {
		return domain.OverlayFile{}, &Error{Code: CodeSourceChanged, Path: relative, Err: err}
	}
	after, err := os.Lstat(path)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || after.Size() != before.Size() || after.Mode().Perm() != before.Mode().Perm() {
		if err == nil {
			err = fmt.Errorf("file metadata changed during discovery")
		}
		return domain.OverlayFile{}, &Error{Code: CodeSourceChanged, Path: relative, Err: err}
	}
	return domain.OverlayFile{Path: relative, Size: size, Mode: uint32(before.Mode().Perm()), SHA256: hash}, nil
}

func hashRegularFile(path string) (string, int64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", 0, fmt.Errorf("not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

// ValidateBaseCollisions rejects exact file collisions and both incompatible
// file/directory ancestor relationships against the selected Git tree.
func ValidateBaseCollisions(snapshot Snapshot, basePaths []string) error {
	base := make(map[string]struct{}, len(basePaths))
	for _, raw := range basePaths {
		base[normalizeGitPath(raw)] = struct{}{}
	}
	for _, file := range snapshot.Files {
		path := normalizeGitPath(file.Path)
		if _, ok := base[path]; ok {
			return &Error{Code: CodeBaseConflict, Path: path, Err: fmt.Errorf("base tree contains the same path")}
		}
		for parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(path))); parent != "." && parent != ""; parent = filepath.ToSlash(filepath.Dir(filepath.FromSlash(parent))) {
			if _, ok := base[parent]; ok {
				return &Error{Code: CodeBaseConflict, Path: path, Err: fmt.Errorf("base tree contains file ancestor %s", parent)}
			}
		}
		prefix := path + "/"
		for candidate := range base {
			if strings.HasPrefix(candidate, prefix) {
				return &Error{Code: CodeBaseConflict, Path: path, Err: fmt.Errorf("base tree contains a descendant below a local file")}
			}
		}
	}
	return nil
}

// VerifySource confirms that a pending snapshot still refers to the same
// source bytes and portable mode bits before any target publication.
func VerifySource(source string, snapshot Snapshot) error {
	for _, expected := range snapshot.Files {
		actual, err := snapshotFile(filepath.Join(source, filepath.FromSlash(expected.Path)), expected.Path, nil)
		if err != nil {
			return err
		}
		if actual != expected {
			return &Error{Code: CodeSourceChanged, Path: expected.Path, Err: fmt.Errorf("source no longer matches the saved snapshot")}
		}
	}
	return nil
}

// CheckDestinations verifies existing target files without creating anything.
// Missing parents/files are returned as missing; any existing incompatible or
// changed path is a conflict.
func CheckDestinations(target string, snapshot Snapshot) ([]DestinationCheck, error) {
	checks := make([]DestinationCheck, 0, len(snapshot.Files))
	for _, expected := range snapshot.Files {
		state, err := checkDestination(target, expected)
		if err != nil {
			return nil, err
		}
		checks = append(checks, DestinationCheck{Path: expected.Path, State: state})
	}
	return checks, nil
}

// HasExistingFiles reports whether any file from a saved overlay currently
// exists in the target. This is used by deletion preflight because Git's
// ordinary porcelain status intentionally hides files ignored by the target
// repository's .gitignore.
func HasExistingFiles(target string, snapshot Snapshot) (bool, error) {
	if err := validateTargetRoot(target); err != nil {
		return false, err
	}
	for _, expected := range snapshot.Files {
		destination := filepath.Join(target, filepath.FromSlash(expected.Path))
		if !fsx.Within(target, destination) {
			return false, &Error{Code: CodeDestinationConflict, Path: expected.Path, Err: fmt.Errorf("destination escapes target")}
		}
		parentState, err := existingDestinationParents(target, expected.Path)
		if err != nil {
			return false, err
		}
		if parentState == "missing" {
			continue
		}
		if _, err := os.Lstat(destination); err == nil {
			return true, nil
		} else if !os.IsNotExist(err) {
			return false, &Error{Code: CodeDestinationConflict, Path: expected.Path, Err: err}
		}
	}
	return false, nil
}

// Materialize copies only missing files. Existing files are accepted only if
// their complete content hash and portable mode bits match the snapshot. A
// destination-local hard-link publish provides atomic no-overwrite semantics
// on the supported local filesystems.
func Materialize(ctx context.Context, source, target string, snapshot Snapshot) (MaterializeResult, error) {
	var result MaterializeResult
	if err := validateTargetRoot(target); err != nil {
		return result, err
	}
	for _, expected := range snapshot.Files {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		state, err := checkDestination(target, expected)
		if err != nil {
			return result, err
		}
		if state == "match" {
			result.Existing++
			continue
		}
		if err := ensureDestinationParents(target, expected.Path); err != nil {
			return result, err
		}
		if err := copyAndPublish(ctx, source, target, expected); err != nil {
			return result, err
		}
		result.Copied++
	}
	return result, nil
}

func checkDestination(target string, expected domain.OverlayFile) (string, error) {
	if err := validateTargetRoot(target); err != nil {
		return "", err
	}
	destination := filepath.Join(target, filepath.FromSlash(expected.Path))
	if !fsx.Within(target, destination) {
		return "", &Error{Code: CodeDestinationConflict, Path: expected.Path, Err: fmt.Errorf("destination escapes target")}
	}
	parentState, err := existingDestinationParents(target, expected.Path)
	if err != nil {
		return "", err
	}
	if parentState == "missing" {
		return "missing", nil
	}
	info, err := os.Lstat(destination)
	if os.IsNotExist(err) {
		return "missing", nil
	}
	if err != nil {
		return "", &Error{Code: CodeDestinationConflict, Path: expected.Path, Err: err}
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", &Error{Code: CodeDestinationConflict, Path: expected.Path, Err: fmt.Errorf("destination is not a regular file")}
	}
	actualHash, actualSize, err := hashRegularFile(destination)
	if err != nil || actualHash != expected.SHA256 || actualSize != expected.Size || uint32(info.Mode().Perm()) != expected.Mode {
		if err == nil {
			err = fmt.Errorf("destination content or mode differs from the snapshot")
		}
		return "", &Error{Code: CodeDestinationConflict, Path: expected.Path, Err: err}
	}
	return "match", nil
}

func validateTargetRoot(target string) error {
	info, err := os.Lstat(target)
	if err != nil {
		return &Error{Code: CodeDestinationConflict, Path: ".", Err: err}
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return &Error{Code: CodeDestinationConflict, Path: ".", Err: fmt.Errorf("target is not a directory")}
	}
	return nil
}

func existingDestinationParents(target, relative string) (string, error) {
	current := target
	parts := strings.Split(filepath.ToSlash(relative), "/")
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return "missing", nil
		}
		if err != nil {
			return "", &Error{Code: CodeDestinationConflict, Path: relative, Err: err}
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", &Error{Code: CodeDestinationConflict, Path: relative, Err: fmt.Errorf("destination parent is not a directory")}
		}
	}
	return "present", nil
}

func ensureDestinationParents(target, relative string) error {
	current := target
	parts := strings.Split(filepath.ToSlash(relative), "/")
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0755); err != nil && !os.IsExist(err) {
				return &Error{Code: CodeCopyFailed, Path: relative, Err: err}
			}
			info, err = os.Lstat(current)
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			if err == nil {
				err = fmt.Errorf("destination parent is not a directory")
			}
			return &Error{Code: CodeDestinationConflict, Path: relative, Err: err}
		}
	}
	return nil
}

func copyAndPublish(ctx context.Context, source, target string, expected domain.OverlayFile) error {
	sourcePath := filepath.Join(source, filepath.FromSlash(expected.Path))
	destination := filepath.Join(target, filepath.FromSlash(expected.Path))
	parent := filepath.Dir(destination)
	temporary, err := os.CreateTemp(parent, ".taskflow-overlay-*")
	if err != nil {
		return &Error{Code: CodeCopyFailed, Path: expected.Path, Err: err}
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	before, err := os.Lstat(sourcePath)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		if err == nil {
			err = fmt.Errorf("source is not a regular file")
		}
		_ = temporary.Close()
		return &Error{Code: CodeSourceChanged, Path: expected.Path, Err: err}
	}
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		_ = temporary.Close()
		return &Error{Code: CodeSourceChanged, Path: expected.Path, Err: err}
	}
	hash := sha256.New()
	tee := io.MultiWriter(temporary, hash)
	size, copyErr := io.Copy(tee, sourceFile)
	closeSourceErr := sourceFile.Close()
	if copyErr != nil || closeSourceErr != nil {
		_ = temporary.Close()
		if copyErr == nil {
			copyErr = closeSourceErr
		}
		return &Error{Code: CodeSourceChanged, Path: expected.Path, Err: copyErr}
	}
	if err := ctx.Err(); err != nil {
		_ = temporary.Close()
		return err
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if size != expected.Size || actualHash != expected.SHA256 {
		_ = temporary.Close()
		return &Error{Code: CodeSourceChanged, Path: expected.Path, Err: fmt.Errorf("source changed while copying")}
	}
	after, err := os.Lstat(sourcePath)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || after.Size() != before.Size() || after.Mode().Perm() != before.Mode().Perm() {
		if err == nil {
			err = fmt.Errorf("source changed while copying")
		}
		_ = temporary.Close()
		return &Error{Code: CodeSourceChanged, Path: expected.Path, Err: err}
	}
	verified, err := snapshotFile(sourcePath, expected.Path, nil)
	if err != nil || verified != expected {
		if err == nil {
			err = fmt.Errorf("source changed before publication")
		}
		_ = temporary.Close()
		return &Error{Code: CodeSourceChanged, Path: expected.Path, Err: err}
	}
	if err := temporary.Chmod(os.FileMode(expected.Mode)); err != nil {
		_ = temporary.Close()
		return &Error{Code: CodeCopyFailed, Path: expected.Path, Err: err}
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return &Error{Code: CodeCopyFailed, Path: expected.Path, Err: err}
	}
	if err := temporary.Close(); err != nil {
		return &Error{Code: CodeCopyFailed, Path: expected.Path, Err: err}
	}

	if err := os.Link(temporaryName, destination); err != nil {
		if os.IsExist(err) {
			state, checkErr := checkDestination(target, expected)
			if checkErr != nil {
				return checkErr
			}
			if state == "match" {
				return nil
			}
		}
		return &Error{Code: CodeDestinationConflict, Path: expected.Path, Err: err}
	}
	return nil
}

func hasGitComponent(path string) bool {
	for _, component := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if component == ".git" {
			return true
		}
	}
	return false
}

func isWindowsAbsolute(path string) bool {
	return len(path) >= 2 && ((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) && path[1] == ':'
}

func normalizeGitPath(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "./" {
		return "."
	}
	return strings.TrimPrefix(path, "./")
}
