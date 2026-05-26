package gitexclude

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/claytercek/offstage/internal/resolver"
)

const (
	beginMarker = "# offstage begin"
	endMarker   = "# offstage end"
)

type Result struct {
	Path         string
	PatternCount int
	Changed      bool
}

// Sync reconciles the offstage-managed block in .git/info/exclude with patterns.
func Sync(repoRoot string, patterns []string) (*Result, error) {
	excludePath, err := resolver.GitPath(repoRoot, "info/exclude")
	if err != nil {
		return nil, fmt.Errorf("resolve git exclude path: %w", err)
	}

	content, err := os.ReadFile(excludePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read git exclude: %w", err)
	}

	next, err := replaceManagedBlock(string(content), patterns)
	if err != nil {
		return nil, err
	}

	if string(content) == next {
		return &Result{Path: excludePath, PatternCount: len(patterns), Changed: false}, nil
	}

	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return nil, fmt.Errorf("create git exclude dir: %w", err)
	}
	if err := os.WriteFile(excludePath, []byte(next), 0o644); err != nil {
		return nil, fmt.Errorf("write git exclude: %w", err)
	}

	return &Result{Path: excludePath, PatternCount: len(patterns), Changed: true}, nil
}

func replaceManagedBlock(content string, patterns []string) (string, error) {
	lines := splitLines(content)
	start, end := -1, -1

	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == beginMarker && start == -1 {
			start = i
			continue
		}
		if trimmed == endMarker && start != -1 {
			end = i
			break
		}
	}

	if start != -1 {
		if end == -1 {
			return "", fmt.Errorf("git exclude contains %q without matching %q", beginMarker, endMarker)
		}
		lines = append(lines[:start], lines[end+1:]...)
	}

	lines = trimTrailingEmpty(lines)
	if len(patterns) == 0 {
		return joinLines(lines), nil
	}

	if len(lines) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines, beginMarker)
	lines = append(lines, patterns...)
	lines = append(lines, endMarker)

	return joinLines(lines), nil
}

func splitLines(content string) []string {
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func trimTrailingEmpty(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
