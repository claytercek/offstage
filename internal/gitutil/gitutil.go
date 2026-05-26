// Package gitutil provides shared git binary detection and execution helpers
// used across offstage packages.
package gitutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Bin returns the path to the git binary. It checks the system PATH first and
// falls back to /usr/bin/git when git is not found on PATH.
func Bin() string {
	if p, err := exec.LookPath("git"); err == nil {
		return p
	}
	return "/usr/bin/git"
}

// Output runs a git command in dir and returns the captured stdout, or an
// error if the command fails. Stderr is forwarded to the process stderr.
func Output(dir string, args ...string) (string, error) {
	cmd := exec.Command(Bin(), args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out.String(), nil
}

// Run runs a git command in dir, forwarding stdout and stderr to the current
// process. Returns an error if the command fails.
func Run(dir string, args ...string) error {
	cmd := exec.Command(Bin(), args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
