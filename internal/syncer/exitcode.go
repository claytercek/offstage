package syncer

// ExitCode returns the process exit code that should be used for err.
//
// Sentinel errors and their exit codes:
//   - nil                → 0 (success)
//   - ErrHasDiff         → 1 (POSIX diff convention: differences found)
//   - ErrBranchNotFound  → 1 (no sync data yet)
//   - ErrDiverged        → 1 (diverged branches)
//   - any other error    → 1 (generic failure)
//
// This is the single place to add or change exit code mappings; command
// handlers in main.go call os.Exit(syncer.ExitCode(err)) without inspecting
// individual sentinel types.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	return 1
}
