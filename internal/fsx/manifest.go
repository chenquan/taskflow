package fsx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManifestName is the whitelist file, at the source repository root, that
// declares which working-directory paths are copied into newly created
// Taskflow worktrees.
const ManifestName = ".taskflowcopy"

// ManifestError distinguishes a missing whitelist from an invalid one so
// create can report a precise preflight diagnostic.
type ManifestError struct {
	Missing bool
	Path    string
	Line    int
	Err     error
}

func (e *ManifestError) Error() string {
	if e.Missing {
		return fmt.Sprintf("source repository has no %s manifest; create one (comments-only declares nothing to copy)", ManifestName)
	}
	if e.Line > 0 {
		return fmt.Sprintf("invalid %s line %d: %v", ManifestName, e.Line, e.Err)
	}
	return fmt.Sprintf("invalid %s: %v", ManifestName, e.Err)
}

func (e *ManifestError) Unwrap() error { return e.Err }

// Pattern is one whitelist line. A trailing `/` restricts the pattern to
// directories; a pattern containing an interior slash is anchored to the
// source root; an unanchored pattern matches a path segment basename at any
// depth. Negation is intentionally unsupported.
type Pattern struct {
	Raw      string
	DirOnly  bool
	Anchored bool
	Literal  bool
	segments []string
}

// Manifest is the parsed whitelist. Matching is additive: the union of all
// pattern matches is copied.
type Manifest struct {
	Path     string
	Patterns []Pattern
}

// LoadManifest reads and validates the whitelist at the source root. It never
// inspects any other source content, so dry-run stays cheap.
func LoadManifest(sourceRoot string) (*Manifest, error) {
	path := filepath.Join(sourceRoot, ManifestName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ManifestError{Missing: true, Path: path}
		}
		return nil, &ManifestError{Path: path, Err: err}
	}
	manifest := &Manifest{Path: path}
	for number, line := range strings.Split(string(raw), "\n") {
		text := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		pattern, err := parsePattern(text)
		if err != nil {
			return nil, &ManifestError{Path: path, Line: number + 1, Err: err}
		}
		manifest.Patterns = append(manifest.Patterns, pattern)
	}
	return manifest, nil
}

func parsePattern(text string) (Pattern, error) {
	pattern := Pattern{Raw: text}
	if strings.HasPrefix(text, "!") {
		return pattern, fmt.Errorf("negation patterns are not supported")
	}
	if strings.HasSuffix(text, "/") {
		pattern.DirOnly = true
		text = strings.TrimRight(text, "/")
		if text == "" {
			return pattern, fmt.Errorf("pattern %q has no path", pattern.Raw)
		}
	}
	if strings.HasPrefix(text, "/") {
		return pattern, fmt.Errorf("pattern %q must be relative to the repository root", pattern.Raw)
	}
	pattern.Anchored = strings.Contains(text, "/")
	segments := strings.Split(text, "/")
	for _, segment := range segments {
		switch segment {
		case "":
			return pattern, fmt.Errorf("pattern %q has an empty path segment", pattern.Raw)
		case ".", "..":
			return pattern, fmt.Errorf("pattern %q must stay inside the repository root", pattern.Raw)
		}
	}
	pattern.segments = segments
	pattern.Literal = !strings.ContainsAny(text, `*?[\`)
	return pattern, nil
}

// Select reports whether the slash-separated source-root-relative path is
// copied, recording literal-pattern hits so unmatched patterns can be
// reported after the copy walk.
func (m *Manifest) Select(path string, isDir bool, literalHits []bool) bool {
	segments := strings.Split(path, "/")
	matched := false
	for i := range m.Patterns {
		pattern := &m.Patterns[i]
		if !pattern.matchesPath(segments, isDir) {
			continue
		}
		matched = true
		if pattern.Literal && literalHits != nil {
			literalHits[i] = true
		}
	}
	return matched
}

func (p *Pattern) matchesPath(segments []string, isDir bool) bool {
	if p.DirOnly && !isDir {
		return false
	}
	if !p.Anchored {
		if len(segments) == 0 {
			return false
		}
		matched, _ := filepath.Match(p.segments[len(p.segments)-1], segments[len(segments)-1])
		return matched
	}
	return matchSegments(p.segments, segments)
}

// matchSegments matches anchored patterns where `**` spans zero or more path
// segments and the other globs stay within one segment.
func matchSegments(pattern, path []string) bool {
	for len(pattern) > 0 {
		if pattern[0] == "**" {
			if len(pattern) == 1 {
				return true
			}
			for i := 0; i <= len(path); i++ {
				if matchSegments(pattern[1:], path[i:]) {
					return true
				}
			}
			return false
		}
		if len(path) == 0 {
			return false
		}
		matched, _ := filepath.Match(pattern[0], path[0])
		if !matched {
			return false
		}
		pattern, path = pattern[1:], path[1:]
	}
	return len(path) == 0
}
