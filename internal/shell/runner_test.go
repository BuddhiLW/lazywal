package shell_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
)

func TestExitCode(t *testing.T) {
	c := shell.Pkill("mpv")
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"exit error", &shell.ExitError{Cmd: c, Code: 1}, 1},
		{"exit error code 2", &shell.ExitError{Cmd: c, Code: 2}, 2},
		{"wrapped exit error", fmt.Errorf("killing: %w", &shell.ExitError{Cmd: c, Code: 3}), 3},
		{"fake exit", shelltest.Exit(c, 137), 137},
		{"other error", errors.New("exec: not found"), -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shell.ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestExitErrorMessage(t *testing.T) {
	c := shell.Command("pkill", "-x", "mpv")
	tests := []struct {
		output string
		want   string
	}{
		{"", "pkill -x mpv: exit status 2"},
		{" \n", "pkill -x mpv: exit status 2"},
		{"  pkill: bad option\n", "pkill -x mpv: exit status 2: pkill: bad option"},
	}
	for _, tt := range tests {
		err := &shell.ExitError{Cmd: c, Code: 2, Output: tt.output}
		if got := err.Error(); got != tt.want {
			t.Errorf("Error() with output %q = %q, want %q", tt.output, got, tt.want)
		}
	}
}

func TestKillByName(t *testing.T) {
	pkill := shell.Pkill("mpv")
	notFound := errors.New("pkill: not found")
	tests := []struct {
		name     string
		err      error // what pkill returns
		wantCode int   // ExitCode of KillByName's result
	}{
		{"killed something", nil, 0},
		{"nothing matched", shelltest.Exit(pkill, 1), 0},
		{"syntax error", shelltest.Exit(pkill, 2), 2},
		{"fatal error", shelltest.Exit(pkill, 3), 3},
		{"pkill not runnable", notFound, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := shelltest.New()
			if tt.err != nil {
				f.Errors[pkill.String()] = tt.err
			}
			err := shell.KillByName(f, "mpv")
			if got := shell.ExitCode(err); got != tt.wantCode {
				t.Errorf("KillByName() = %v (code %d), want code %d", err, got, tt.wantCode)
			}
			if tt.wantCode != 0 && !errors.Is(err, tt.err) {
				t.Errorf("KillByName() = %v, want %v propagated", err, tt.err)
			}
			if n := f.Count(pkill.String()); n != 1 {
				t.Errorf("ran pkill %d times, want 1: %q", n, f.Commands())
			}
		})
	}
}

func TestIsRunning(t *testing.T) {
	pgrep := shell.Pgrep("mpvpaper")
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"found", nil, true},
		{"none", shelltest.Exit(pgrep, 1), false},
		{"pgrep failed", shelltest.Exit(pgrep, 2), false},
		{"pgrep not runnable", errors.New("pgrep: not found"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := shelltest.New()
			if tt.err != nil {
				f.Errors[pgrep.String()] = tt.err
			}
			if got := shell.IsRunning(f, "mpvpaper"); got != tt.want {
				t.Errorf("IsRunning() = %v, want %v", got, tt.want)
			}
			if !f.Ran(pgrep.String()) {
				t.Errorf("pgrep not run: %q", f.Commands())
			}
		})
	}
}
