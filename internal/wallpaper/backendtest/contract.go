// Package backendtest checks that a wallpaper.Backend honours the contract
// documented on the interface, so every implementation can be substituted
// for any other (Liskov substitution).
package backendtest

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

// Video creates a real (empty) file and returns it as a Video. Its name has
// spaces and a quote to catch implementations that build shell strings.
func Video(t testing.TB) wallpaper.Video {
	t.Helper()
	path := filepath.Join(t.TempDir(), `it's a "video".mp4`)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := wallpaper.NewVideo(path)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// Contract runs the Backend contract against fresh backends from newBackend.
// newBackend must return a backend wired to fakes (shelltest.Fake and an
// in-memory store) whose dependencies are all present.
func Contract(t *testing.T, newBackend func(t *testing.T) wallpaper.Backend) {
	t.Run("identity", func(t *testing.T) {
		b := newBackend(t)
		name := b.Name()
		if name == "" || name != strings.ToLower(name) || strings.ContainsAny(name, " \t\n") {
			t.Errorf("Name() = %q, want non-empty, lower-case, no spaces", name)
		}
		if strings.TrimSpace(b.Describe()) == "" {
			t.Error("Describe() is empty")
		}
	})

	t.Run("check passes when dependencies are present", func(t *testing.T) {
		if err := newBackend(t).Check(); err != nil {
			t.Errorf("Check() = %v, want nil", err)
		}
	})

	t.Run("clear with nothing set succeeds", func(t *testing.T) {
		b := newBackend(t)
		if err := b.Clear(); err != nil {
			t.Errorf("Clear() before Set = %v", err)
		}
		if err := b.Clear(); err != nil {
			t.Errorf("second Clear() = %v", err)
		}
	})

	t.Run("set replaces without clear", func(t *testing.T) {
		b := newBackend(t)
		v := Video(t)
		if err := b.Set(v); err != nil {
			t.Fatalf("Set() = %v", err)
		}
		if err := b.Set(v); err != nil {
			t.Fatalf("second Set() = %v", err)
		}
	})

	t.Run("clear after set is idempotent", func(t *testing.T) {
		b := newBackend(t)
		if err := b.Set(Video(t)); err != nil {
			t.Fatalf("Set() = %v", err)
		}
		if err := b.Clear(); err != nil {
			t.Fatalf("Clear() = %v", err)
		}
		if err := b.Clear(); err != nil {
			t.Fatalf("second Clear() = %v", err)
		}
	})
}

// ContractWithRunner runs Contract, then checks through the fake runner what
// the backend does to processes, which return values alone cannot show: a
// backend that stacks a new player on every Set, or leaves one running after
// Clear, still returns nil everywhere.
//
//   - Check starts, kills and pkills nothing.
//   - A second Set stops every process the first Set started.
//   - Clear stops every process Set started. Processes Clear itself starts
//     (e.g. a restored desktop) are meant to outlive it and are not checked.
//
// A process counts as stopped when its PID was passed to KillGroup, or when
// a "pkill ... -x <name>" ran after it started, where name is the base name
// of the program Start was given.
//
// newBackend must wire the backend to r and s, script r so every dependency
// is present and Set can succeed, and may script more (outputs, errors).
func ContractWithRunner(t *testing.T, newBackend func(t *testing.T, r *shelltest.Fake, s *shelltest.Store) wallpaper.Backend) {
	Contract(t, func(t *testing.T) wallpaper.Backend {
		return newBackend(t, shelltest.New(), shelltest.NewStore())
	})

	t.Run("check touches no process", func(t *testing.T) {
		r := shelltest.New()
		b := newBackend(t, r, shelltest.NewStore())
		calls, started, killed := len(r.Calls), len(r.Started), len(r.Killed)
		if err := b.Check(); err != nil {
			t.Fatalf("Check() = %v", err)
		}
		if n := len(r.Started) - started; n != 0 {
			t.Errorf("Check started %d processes: %q", n, r.Started[started:])
		}
		if n := len(r.Killed) - killed; n != 0 {
			t.Errorf("Check killed process groups %v", r.Killed[killed:])
		}
		for _, c := range r.Calls[calls:] {
			if filepath.Base(c.Name) == "pkill" {
				t.Errorf("Check ran %q", c)
			}
		}
	})

	t.Run("set stops what the previous set started", func(t *testing.T) {
		r := shelltest.New()
		b := newBackend(t, r, shelltest.NewStore())
		v := Video(t)
		first := len(r.Procs)
		if err := b.Set(v); err != nil {
			t.Fatalf("Set() = %v", err)
		}
		procs := slices.Clone(r.Procs[first:])
		if err := b.Set(v); err != nil {
			t.Fatalf("second Set() = %v", err)
		}
		for _, p := range procs {
			if !stopped(r, p) {
				t.Errorf("second Set left %s (PID %d) from the first Set running: %q", filepath.Base(p.Cmd.Name), p.PID, p.Cmd)
			}
		}
	})

	t.Run("clear stops what set started", func(t *testing.T) {
		r := shelltest.New()
		b := newBackend(t, r, shelltest.NewStore())
		first := len(r.Procs)
		if err := b.Set(Video(t)); err != nil {
			t.Fatalf("Set() = %v", err)
		}
		procs := slices.Clone(r.Procs[first:])
		if err := b.Clear(); err != nil {
			t.Fatalf("Clear() = %v", err)
		}
		for _, p := range procs {
			if !stopped(r, p) {
				t.Errorf("Clear left %s (PID %d) running: %q", filepath.Base(p.Cmd.Name), p.PID, p.Cmd)
			}
		}
	})
}

// stopped reports whether p was killed by PID, or by a pkill -x naming its
// program that ran after p started.
func stopped(r *shelltest.Fake, p shelltest.Proc) bool {
	if slices.Contains(r.Killed, p.PID) {
		return true
	}
	name := filepath.Base(p.Cmd.Name)
	for _, c := range r.Calls[p.Call+1:] {
		if filepath.Base(c.Name) != "pkill" {
			continue
		}
		if i := slices.Index(c.Args, "-x"); i >= 0 && i+1 < len(c.Args) && c.Args[i+1] == name {
			return true
		}
	}
	return false
}

// MissingDeps asserts that err is a *MissingDepsError listing exactly want
// (order-insensitive), i.e. that Check reports every missing dependency.
func MissingDeps(t *testing.T, err error, want ...string) {
	t.Helper()
	var m *wallpaper.MissingDepsError
	if !errors.As(err, &m) {
		t.Fatalf("Check() = %v, want *MissingDepsError", err)
	}
	got := map[string]bool{}
	for _, d := range m.Deps {
		got[d] = true
	}
	for _, w := range want {
		found := got[w]
		if !found { // allow descriptive entries that start with the name
			for d := range got {
				if strings.HasPrefix(d, w) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("missing deps %q do not mention %q", m.Deps, w)
		}
	}
	if len(m.Deps) != len(want) {
		t.Errorf("missing deps = %q, want %d entries (%q)", m.Deps, len(want), want)
	}
}
