package wallpaper_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

func missingDeps(t *testing.T, err error) []string {
	t.Helper()
	var m *wallpaper.MissingDepsError
	if !errors.As(err, &m) {
		t.Fatalf("Err() = %v, want *MissingDepsError", err)
	}
	return m.Deps
}

func TestRequirementsPrograms(t *testing.T) {
	f := shelltest.New()
	f.Missing["xwinwrap"] = true
	f.Missing["mpvpaper"] = true
	r := wallpaper.Require(f.LookPath)

	if !r.Programs() {
		t.Error("Programs() with no names = false")
	}
	if !r.Programs("mpv") {
		t.Error("Programs(mpv) = false, want true")
	}
	if r.Err() != nil {
		t.Errorf("Err() = %v with nothing missing yet", r.Err())
	}
	if r.Programs("xwinwrap", "ffmpeg", "mpvpaper") {
		t.Error("Programs(xwinwrap, ffmpeg, mpvpaper) = true, want false")
	}

	// Every missing program is reported, not only the first, in call order.
	backendtest.MissingDeps(t, r.Err(), "xwinwrap", "mpvpaper")
	if got := missingDeps(t, r.Err()); !slices.Equal(got, []string{"xwinwrap", "mpvpaper"}) {
		t.Errorf("deps = %q", got)
	}
}

func TestRequirementsThat(t *testing.T) {
	r := wallpaper.Require(shelltest.New().LookPath)
	if !r.That(true, "gsettings schema") {
		t.Error("That(true) = false")
	}
	if r.That(false, "a running GNOME Shell") {
		t.Error("That(false) = true")
	}
	backendtest.MissingDeps(t, r.Err(), "a running GNOME Shell")
}

func TestRequirementsInclude(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want []string
	}{
		{"nil adds nothing", nil, nil},
		{"missing deps are merged",
			&wallpaper.MissingDepsError{Deps: []string{"mpv", "ffmpeg"}}, []string{"mpv", "ffmpeg"}},
		{"wrapped missing deps are merged",
			fmt.Errorf("xwinwrap backend: %w", &wallpaper.MissingDepsError{Deps: []string{"mpv"}}), []string{"mpv"}},
		{"plain error is added as text",
			errors.New("no X display"), []string{"no X display"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := wallpaper.Require(shelltest.New().LookPath)
			r.That(false, "earlier")
			r.Include(tt.err)
			want := append([]string{"earlier"}, tt.want...)
			if got := missingDeps(t, r.Err()); !slices.Equal(got, want) {
				t.Errorf("deps = %q, want %q", got, want)
			}
		})
	}
}

func TestRequirementsIncludeAnotherCheck(t *testing.T) {
	f := shelltest.New()
	f.Missing["wal"] = true
	inner := wallpaper.Require(f.LookPath)
	inner.Programs("wal", "ffmpeg")

	outer := wallpaper.Require(f.LookPath)
	outer.Include(inner.Err())
	outer.That(false, "gnome-shell extension")
	backendtest.MissingDeps(t, outer.Err(), "wal", "gnome-shell extension")

	clean := wallpaper.Require(f.LookPath)
	clean.Include(wallpaper.Require(f.LookPath).Err())
	if err := clean.Err(); err != nil {
		t.Errorf("including a passing check: Err() = %v", err)
	}
}

func TestRequirementsErr(t *testing.T) {
	r := wallpaper.Require(shelltest.New().LookPath)
	if err := r.Err(); err != nil {
		t.Fatalf("fresh Err() = %v, want nil", err)
	}

	r.That(false, "a")
	first := r.Err()
	r.That(false, "b")
	second := r.Err()

	if got := missingDeps(t, first); !slices.Equal(got, []string{"a"}) {
		t.Errorf("earlier Err() changed to %q", got)
	}
	secondDeps := missingDeps(t, second)
	secondDeps[0] = "MUTATED"
	if got := missingDeps(t, r.Err()); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("Err() = %q after mutating a returned error", got)
	}
	if got, want := r.Err().Error(), "missing dependencies: a, b"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
