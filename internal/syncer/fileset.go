package syncer

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileSet provides unified file tracking across push and pull operations.
// It encapsulates glob matching, file enumeration, and file copying.
type FileSet struct{}

// Collect walks dir and returns relative paths of files matching any include
// pattern and not matching any exclude pattern.
func (FileSet) Collect(dir string, include []string, exclude []string) ([]string, error) {
	var result []string

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		if matchesAny(relPath, include) && !matchesAny(relPath, exclude) {
			result = append(result, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Matches reports whether path matches the given glob pattern.
// Supports ** for recursive directory matching.
func (FileSet) Matches(pattern, path string) bool {
	return matchGlob(pattern, path)
}

// Copy copies the file at src to dst, creating dst if it does not exist.
func (FileSet) Copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// matchesAny returns true if path matches any of the glob patterns.
func matchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchGlob(pattern, path) {
			return true
		}
	}
	return false
}

// matchGlob matches a single pattern against path, supporting ** for
// recursive directory matching.
func matchGlob(pattern, path string) bool {
	if !strings.Contains(pattern, "**") {
		ok, _ := filepath.Match(pattern, path)
		return ok
	}

	// Split on "/**" to get prefix and suffix.
	parts := strings.SplitN(pattern, "/**", 2)
	prefix := parts[0]

	if prefix == "" {
		// Pattern is "**" or "/**..." — matches everything.
		return true
	}

	// path must start with prefix+"/" or equal prefix.
	return strings.HasPrefix(path, prefix+"/") || path == prefix
}
