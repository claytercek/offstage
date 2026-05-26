package syncer_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/claytercek/offstage/internal/syncer"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "nil error",
			err:      nil,
			wantCode: 0,
		},
		{
			name:     "ErrHasDiff",
			err:      syncer.ErrHasDiff,
			wantCode: 1,
		},
		{
			name:     "ErrBranchNotFound",
			err:      syncer.ErrBranchNotFound,
			wantCode: 1,
		},
		{
			name:     "ErrDiverged",
			err:      syncer.ErrDiverged,
			wantCode: 1,
		},
		{
			name:     "wrapped ErrHasDiff",
			err:      fmt.Errorf("outer: %w", syncer.ErrHasDiff),
			wantCode: 1,
		},
		{
			name:     "wrapped ErrBranchNotFound",
			err:      fmt.Errorf("outer: %w", syncer.ErrBranchNotFound),
			wantCode: 1,
		},
		{
			name:     "wrapped ErrDiverged",
			err:      fmt.Errorf("outer: %w", syncer.ErrDiverged),
			wantCode: 1,
		},
		{
			name:     "unknown error",
			err:      errors.New("some other error"),
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := syncer.ExitCode(tt.err)
			if got != tt.wantCode {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.wantCode)
			}
		})
	}
}
