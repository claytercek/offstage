package syncer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

type patternMatcher interface {
	MatchesPath(string) bool
}

// FileSet provides unified file tracking across push and pull operations.
// It encapsulates pattern matching, file enumeration, and file copying.
type FileSet struct{}

// Collect walks dir and returns relative paths of files matching any include
// pattern and not matching any exclude pattern.
func (FileSet) Collect(dir string, include []string, exclude []string) ([]string, error) {
	var result []string
	includeMatcher := compilePatterns(include)
	excludeMatcher := compilePatterns(exclude)

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if relPath == ".git" || strings.HasPrefix(relPath, ".git"+string(filepath.Separator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		if matchesAny(relPath, includeMatcher) && !matchesAny(relPath, excludeMatcher) {
			result = append(result, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Matches reports whether path matches the given git-ignore-compatible pattern.
func (FileSet) Matches(pattern, path string) bool {
	return compilePatterns([]string{pattern}).MatchesPath(path)
}

// Copy copies the file at src to dst, creating dst if it does not exist.
func (FileSet) Copy(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := in.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close source file: %w", closeErr))
		}
	}()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close destination file: %w", closeErr))
		}
	}()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func compilePatterns(patterns []string) patternMatcher {
	return ignore.CompileIgnoreLines(patterns...)
}

func matchesAny(path string, matcher patternMatcher) bool {
	return matcher.MatchesPath(path)
}
