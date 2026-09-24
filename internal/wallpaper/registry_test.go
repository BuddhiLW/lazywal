package wallpaper_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

var (
	x11Bare    = wallpaper.Session{Type: wallpaper.X11}
	x11Gnome   = wallpaper.Session{Type: wallpaper.X11, Desktops: []string{"ubuntu", "gnome"}}
	x11Budgie  = wallpaper.Session{Type: wallpaper.X11, Desktops: []string{"budgie", "gnome"}}
	x11Xfce    = wallpaper.Session{Type: wallpaper.X11, Desktops: []string{"xfce"}}
	waylandKDE = wallpaper.Session{Type: wallpaper.Wayland, Desktops: []string{"kde"}}
)

func TestRuleMatches(t *testing.T) {
	tests := []struct {
		name string
		rule wallpaper.Rule
		s    wallpaper.Session
		want bool
	}{
		{"zero rule matches an unknown session", wallpaper.Rule{}, wallpaper.Session{}, true},
		{"zero rule matches any session", wallpaper.Rule{}, x11Gnome, true},
		{"session filter matches", wallpaper.Rule{Session: wallpaper.X11}, x11Gnome, true},
		{"session filter rejects other type", wallpaper.Rule{Session: wallpaper.X11}, waylandKDE, false},
		{"session filter rejects unknown type", wallpaper.Rule{Session: wallpaper.Wayland}, wallpaper.Session{}, false},
		{"desktops are any-of", wallpaper.Rule{Desktops: []string{"kde", "gnome"}}, x11Gnome, true},
		{"desktops none present", wallpaper.Rule{Desktops: []string{"kde"}}, x11Gnome, false},
		{"desktops need a desktop", wallpaper.Rule{Desktops: []string{"gnome"}}, x11Bare, false},
		{"desktops are case-insensitive", wallpaper.Rule{Desktops: []string{"GNOME"}}, x11Gnome, true},
		{"exclude rejects", wallpaper.Rule{Desktops: []string{"gnome"}, Exclude: []string{"budgie"}}, x11Budgie, false},
		{"exclude is case-insensitive", wallpaper.Rule{Exclude: []string{"Budgie"}}, x11Budgie, false},
		{"exclude absent passes", wallpaper.Rule{Desktops: []string{"gnome"}, Exclude: []string{"budgie"}}, x11Gnome, true},
		{"exclude alone passes other desktops", wallpaper.Rule{Exclude: []string{"gnome"}}, x11Bare, true},
		{"session and desktop both required", wallpaper.Rule{Session: wallpaper.Wayland, Desktops: []string{"gnome"}}, x11Gnome, false},
		{"priority does not affect matching", wallpaper.Rule{Priority: -100}, x11Bare, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.Matches(tt.s); got != tt.want {
				t.Errorf("%+v.Matches(%s) = %v, want %v", tt.rule, tt.s, got, tt.want)
			}
		})
	}
}

func TestCandidatesOrder(t *testing.T) {
	// Repeat so map iteration order cannot make a wrong sort pass by luck.
	for range 20 {
		r := wallpaper.NewRegistry()
		r.Register(&fakeBackend{name: "low-first"}, wallpaper.Rule{})
		r.Register(&fakeBackend{name: "high-a"}, wallpaper.Rule{Priority: 10})
		r.Register(&fakeBackend{name: "wayland-only"}, wallpaper.Rule{Session: wallpaper.Wayland, Priority: 100})
		r.Register(&fakeBackend{name: "high-b"}, wallpaper.Rule{Priority: 10})
		r.Register(&fakeBackend{name: "low-second"}, wallpaper.Rule{})
		r.Register(&fakeBackend{name: "negative"}, wallpaper.Rule{Priority: -5})
		r.Register(&fakeBackend{name: "no-rules"})

		if got, want := names(r.Candidates(x11Bare)), []string{"high-a", "high-b", "low-first", "low-second", "negative"}; !slices.Equal(got, want) {
			t.Fatalf("Candidates(x11) = %v, want %v", got, want)
		}
		if got, want := names(r.Candidates(waylandKDE)), []string{"wayland-only", "high-a", "high-b", "low-first", "low-second", "negative"}; !slices.Equal(got, want) {
			t.Fatalf("Candidates(wayland) = %v, want %v", got, want)
		}
	}
}

func TestCandidatesUseBestMatchingRule(t *testing.T) {
	r := wallpaper.NewRegistry()
	r.Register(&fakeBackend{name: "multi"},
		wallpaper.Rule{Priority: 1},
		wallpaper.Rule{Desktops: []string{"gnome"}, Priority: 50},
		wallpaper.Rule{Session: wallpaper.Wayland, Priority: 99},
		wallpaper.Rule{Priority: -3})
	r.Register(&fakeBackend{name: "mid"}, wallpaper.Rule{Priority: 20})

	tests := []struct {
		s    wallpaper.Session
		want []string
	}{
		{x11Gnome, []string{"multi", "mid"}},   // 50 > 20
		{x11Xfce, []string{"mid", "multi"}},    // only priority 1 and -3 match
		{waylandKDE, []string{"multi", "mid"}}, // 99 > 20
	}
	for _, tt := range tests {
		if got := names(r.Candidates(tt.s)); !slices.Equal(got, tt.want) {
			t.Errorf("Candidates(%s) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestCandidatesNegativePriorities(t *testing.T) {
	r := wallpaper.NewRegistry()
	r.Register(&fakeBackend{name: "minus5"}, wallpaper.Rule{Priority: -5})
	r.Register(&fakeBackend{name: "minus2"}, wallpaper.Rule{Priority: -10}, wallpaper.Rule{Priority: -2})
	r.Register(&fakeBackend{name: "zero"}, wallpaper.Rule{})

	if got, want := names(r.Candidates(x11Bare)), []string{"zero", "minus2", "minus5"}; !slices.Equal(got, want) {
		t.Errorf("Candidates() = %v, want %v", got, want)
	}
}

func TestRegisterReplaces(t *testing.T) {
	r := wallpaper.NewRegistry()
	r.Register(&fakeBackend{name: "a"}, wallpaper.Rule{})
	r.Register(&fakeBackend{name: "b"}, wallpaper.Rule{})
	a2 := &fakeBackend{name: "a"}
	r.Register(a2, wallpaper.Rule{Session: wallpaper.Wayland})

	if got := r.All(); len(got) != 2 {
		t.Fatalf("All() = %v, want 2 backends", names(got))
	}
	if b, err := r.Get("a"); err != nil || b != a2 {
		t.Errorf("Get(a) = %v, %v; want the replacement", b, err)
	}
	// The replacement's rules apply, not the original's.
	if got := names(r.Candidates(x11Bare)); !slices.Equal(got, []string{"b"}) {
		t.Errorf("Candidates(x11) = %v, want [b]", got)
	}
	// It keeps the original tie-break position.
	got := r.Candidates(waylandKDE)
	if !slices.Equal(names(got), []string{"a", "b"}) || got[0] != a2 {
		t.Errorf("Candidates(wayland) = %v, want [a b] with the replacement first", names(got))
	}
}

func TestRegistryGet(t *testing.T) {
	r := wallpaper.NewRegistry()
	xw := &fakeBackend{name: "xwinwrap"}
	r.Register(xw)
	r.Register(&fakeBackend{name: "gnome"})

	for _, name := range []string{"xwinwrap", "XWINWRAP", "  XWinWrap\t"} {
		if b, err := r.Get(name); err != nil || b != xw {
			t.Errorf("Get(%q) = %v, %v; want xwinwrap", name, b, err)
		}
	}
	for _, name := range []string{"nope", ""} {
		b, err := r.Get(name)
		if b != nil || err == nil {
			t.Fatalf("Get(%q) = %v, %v; want error", name, b, err)
		}
		if msg := err.Error(); !strings.Contains(msg, `"`+name+`"`) || !strings.Contains(msg, "available: gnome, xwinwrap") {
			t.Errorf("Get(%q) error = %q, want it to name the input and list available backends", name, msg)
		}
	}
}

func TestRegistryAllSortedByName(t *testing.T) {
	r := wallpaper.NewRegistry()
	for _, n := range []string{"zeta", "alpha", "mu"} {
		r.Register(&fakeBackend{name: n}, wallpaper.Rule{})
	}
	if got := names(r.All()); !slices.Equal(got, []string{"alpha", "mu", "zeta"}) {
		t.Errorf("All() = %v", got)
	}
	if got := wallpaper.NewRegistry().All(); len(got) != 0 {
		t.Errorf("empty All() = %v", names(got))
	}
}

func TestSelectOverride(t *testing.T) {
	r := wallpaper.NewRegistry()
	ready := &fakeBackend{name: "ready"}
	broken := &fakeBackend{name: "broken", checkErr: errors.New("mpvpaper missing")}
	r.Register(ready, wallpaper.Rule{Priority: 10})
	r.Register(broken, wallpaper.Rule{Session: wallpaper.Wayland})

	// The user's choice wins even over a ready candidate and even when the
	// session does not match; readiness is the caller's concern.
	b, err := r.Select(x11Bare, "Broken")
	if err != nil || b != broken {
		t.Fatalf("Select(override broken) = %v, %v", b, err)
	}
	if broken.checks != 0 || ready.checks != 0 {
		t.Errorf("override ran Check (broken %d, ready %d)", broken.checks, ready.checks)
	}

	b, err = r.Select(x11Bare, "nope")
	if b != nil || err == nil || !strings.Contains(err.Error(), "unknown backend") {
		t.Errorf("Select(override nope) = %v, %v; want unknown backend error", b, err)
	}
}

func TestSelectFirstReady(t *testing.T) {
	var journal []string
	r := wallpaper.NewRegistry()
	r.Register(&fakeBackend{name: "best", checkErr: errors.New("not installed"), journal: &journal}, wallpaper.Rule{Priority: 2})
	second := &fakeBackend{name: "second", journal: &journal}
	r.Register(second, wallpaper.Rule{Priority: 1})
	r.Register(&fakeBackend{name: "third", journal: &journal}, wallpaper.Rule{})

	b, err := r.Select(x11Bare, "")
	if err != nil || b != second {
		t.Fatalf("Select() = %v, %v; want second", b, err)
	}
	if want := []string{"best.check", "second.check"}; !slices.Equal(journal, want) {
		t.Errorf("checks = %v, want %v (stop at the first ready)", journal, want)
	}
}

func TestSelectNotReady(t *testing.T) {
	bestErr := &wallpaper.MissingDepsError{Deps: []string{"hanabi"}}
	otherErr := errors.New("xwinwrap missing")
	r := wallpaper.NewRegistry()
	best := &fakeBackend{name: "gnome", checkErr: bestErr}
	r.Register(&fakeBackend{name: "xwinwrap", checkErr: otherErr}, wallpaper.Rule{Session: wallpaper.X11})
	r.Register(best, wallpaper.Rule{Desktops: []string{"gnome"}, Priority: 10})

	b, err := r.Select(x11Gnome, "")
	if b != best {
		t.Errorf("Select() backend = %v, want the best candidate gnome", b)
	}
	var notReady *wallpaper.NotReadyError
	if !errors.As(err, &notReady) {
		t.Fatalf("Select() error = %v, want *NotReadyError", err)
	}
	if notReady.Backend != best || notReady.Err != error(bestErr) || !slices.Equal(notReady.Session.Desktops, x11Gnome.Desktops) {
		t.Errorf("NotReadyError = %+v, want best candidate and its Check error", notReady)
	}
	if !errors.Is(err, bestErr) || errors.Is(err, otherErr) {
		t.Errorf("Select() error %v should unwrap to the best candidate's Check error only", err)
	}
	var missing *wallpaper.MissingDepsError
	if !errors.As(err, &missing) || !slices.Equal(missing.Deps, []string{"hanabi"}) {
		t.Errorf("errors.As(MissingDepsError) through NotReadyError = %v", missing)
	}
	for _, want := range []string{"gnome", x11Gnome.String(), "hanabi"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Error() = %q, want it to mention %q", err, want)
		}
	}
}

func TestSelectNoCandidates(t *testing.T) {
	r := wallpaper.NewRegistry()
	r.Register(&fakeBackend{name: "plasma"}, wallpaper.Rule{Desktops: []string{"kde"}})

	for _, reg := range []*wallpaper.Registry{r, wallpaper.NewRegistry()} {
		b, err := reg.Select(x11Xfce, "")
		if b != nil || err == nil {
			t.Fatalf("Select() = %v, %v; want error", b, err)
		}
		var notReady *wallpaper.NotReadyError
		if errors.As(err, &notReady) {
			t.Errorf("no candidates reported as NotReadyError: %v", err)
		}
		if !strings.Contains(err.Error(), "no backend supports") || !strings.Contains(err.Error(), x11Xfce.String()) {
			t.Errorf("Select() error = %q", err)
		}
	}
}
