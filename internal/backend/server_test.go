package backend

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

func TestIsPortInUseError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "syscall errno",
			err:  syscall.EADDRINUSE,
			want: true,
		},
		{
			name: "wrapped errno",
			err:  fmt.Errorf("listen failed: %w", syscall.EADDRINUSE),
			want: true,
		},
		{
			name: "windows bind text",
			err:  errors.New("listen tcp :3001: bind: Only one usage of each socket address"),
			want: true,
		},
		{
			name: "other error",
			err:  errors.New("permission denied"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPortInUseError(tt.err); got != tt.want {
				t.Fatalf("isPortInUseError() = %v, want %v", got, tt.want)
			}
		})
	}
}
