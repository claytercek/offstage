package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// projectConfigName is a minimal struct for reading the name field from
// .offstagerc.toml without importing the full manifest package.
type projectConfigName struct {
	Name string `toml:"name"`
}

// ResolveProjectID returns the normalized project identifier for the git repo
// containing dir. It checks for a name override in .offstagerc.toml first,
// then falls back to normalizing the "origin" remote URL.
func ResolveProjectID(dir string) (string, error) {
	repoRoot, err := gitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}

	// Check for name override in .offstagerc.toml at the repo root.
	rcPath := filepath.Join(repoRoot, ".offstagerc.toml")
	if _, statErr := os.Stat(rcPath); statErr == nil {
		var cfg projectConfigName
		if _, decodeErr := toml.DecodeFile(rcPath, &cfg); decodeErr == nil && cfg.Name != "" {
			return cfg.Name, nil
		}
	}

	// Derive project ID from the "origin" remote URL.
	remoteURL, err := gitOutput(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("no remote named \"origin\"; run 'offstage init <git-url>' to configure one")
	}

	return normalizeRemoteURL(remoteURL), nil
}

// normalizeRemoteURL converts a git remote URL into a canonical project ID.
//
// Rules:
//  1. Strip trailing .git suffix
//  2. Lowercase the result
//  3. Strip leading https:// or http://
//  4. Convert SSH user@host:path to host/path
func normalizeRemoteURL(u string) string {
	u = strings.TrimSpace(u)

	// Handle SSH form: git@github.com:clay/my-project.git
	if idx := strings.Index(u, "@"); idx != -1 {
		// Strip everything up to and including the "@".
		rest := u[idx+1:]
		// Replace the first ":" (host/path separator) with "/".
		rest = strings.Replace(rest, ":", "/", 1)
		u = rest
	} else {
		// Strip leading https:// or http://
		u = strings.TrimPrefix(u, "https://")
		u = strings.TrimPrefix(u, "http://")
	}

	// Strip trailing .git suffix.
	u = strings.TrimSuffix(u, ".git")

	// Lowercase.
	u = strings.ToLower(u)

	return u
}
