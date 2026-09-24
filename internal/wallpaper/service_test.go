package wallpaper_test

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

// serviceFixture has an "x11" backend selected in X11 sessions and a
// "wayland" backend selected in Wayland ones, sharing one call journal.
type serviceFixture struct {
	svc     *wallpaper.Service
	state   *shelltest.Store
	x11     *fakeBackend
	wayland *fakeBackend
	journal *[]string
	logs    *[]string
}

func newServiceFixture(session wallpaper.Session) *serviceFixture {
	journal, logs := &[]string{}, &[]string{}
	fx := &serviceFixture{
		state:   shelltest.NewStore(),
		x11:     &fakeBackend{name: "x11", journal: journal},
		wayland: &fakeBackend{name: "wayland", journal: journal},
		journal: journal,
		logs:    logs,
	}
	r := wallpaper.NewRegistry()
	r.Register(fx.x11, wallpaper.Rule{Session: wallpaper.X11})
	r.Register(fx.wayland, wallpaper.Rule{Session: wallpaper.Wayland})
	fx.svc = &wallpaper.Service{
		Registry: r,
		State:    fx.state,
		Session:  session,
		Logf: func(format string, args ...any) {
			*logs = append(*logs, fmt.Sprintf(format, args...))
		},
	}
	return fx
}

// mutations drops the read-only Check calls from the journal.
func (fx *serviceFixture) mutations() []string {
	return slices.DeleteFunc(slices.Clone(*fx.journal), func(e string) bool {
		return strings.HasSuffix(e, ".check")
	})
}

func (fx *serviceFixture) assertState(t *testing.T, backend, video string) {
	t.Helper()
	if got := fx.state.Get(wallpaper.StateBackend); got != backend {
		t.Errorf("state backend = %q, want %q", got, backend)
	}
	if got := fx.state.Get(wallpaper.StateVideo); got != video {
		t.Errorf("state video = %q, want %q", got, video)
	}
}

func TestServiceSet(t *testing.T) {
	fx := newServiceFixture(x11Bare)
	v := backendtest.Video(t)

	b, err := fx.svc.Set(v)
	if err != nil || b != fx.x11 {
		t.Fatalf("Set() = %v, %v; want x11", b, err)
	}
	if len(fx.x11.sets) != 1 || fx.x11.sets[0] != v {
		t.Errorf("x11 sets = %v", fx.x11.sets)
	}
	if got := fx.mutations(); !slices.Equal(got, []string{"x11.set"}) {
		t.Errorf("calls = %v, want only x11.set", got)
	}
	fx.assertState(t, "x11", v.Path())
	if !slices.ContainsFunc(*fx.logs, func(l string) bool { return strings.Contains(l, "x11") }) {
		t.Errorf("logs = %q, want the backend used", *fx.logs)
	}
}

func TestServiceSetRejectsZeroVideo(t *testing.T) {
	fx := newServiceFixture(x11Bare)
	if b, err := fx.svc.Set(wallpaper.Video{}); b != nil || err == nil {
		t.Fatalf("Set(zero) = %v, %v; want error", b, err)
	}
	if len(*fx.journal) != 0 {
		t.Errorf("Set(zero) touched backends: %v", *fx.journal)
	}
	fx.assertState(t, "", "")
}

func TestServiceSetChecksReadiness(t *testing.T) {
	missing := &wallpaper.MissingDepsError{Deps: []string{"mpvpaper"}}

	t.Run("override that is not ready", func(t *testing.T) {
		fx := newServiceFixture(x11Bare)
		fx.wayland.checkErr = missing
		fx.svc.Override = "wayland"

		b, err := fx.svc.Set(backendtest.Video(t))
		if b != nil || !errors.Is(err, missing) || !strings.Contains(err.Error(), "wayland") {
			t.Fatalf("Set() = %v, %v; want the override's Check error", b, err)
		}
		if got := fx.mutations(); len(got) != 0 {
			t.Errorf("calls = %v, want none", got)
		}
		fx.assertState(t, "", "")
	})

	t.Run("no candidate ready", func(t *testing.T) {
		fx := newServiceFixture(x11Bare)
		fx.x11.checkErr = missing

		b, err := fx.svc.Set(backendtest.Video(t))
		var notReady *wallpaper.NotReadyError
		if b != nil || !errors.As(err, &notReady) {
			t.Fatalf("Set() = %v, %v; want *NotReadyError", b, err)
		}
		if got := fx.mutations(); len(got) != 0 {
			t.Errorf("calls = %v, want none", got)
		}
		fx.assertState(t, "", "")
	})

	t.Run("previous wallpaper kept when the new backend is not ready", func(t *testing.T) {
		fx := newServiceFixture(x11Bare)
		fx.state.Set(wallpaper.StateBackend, "x11")
		fx.state.Set(wallpaper.StateVideo, "/old.mp4")
		fx.wayland.checkErr = missing
		fx.svc.Override = "wayland"

		if _, err := fx.svc.Set(backendtest.Video(t)); err == nil {
			t.Fatal("Set() = nil error")
		}
		if got := fx.mutations(); len(got) != 0 {
			t.Errorf("calls = %v, want none", got)
		}
		fx.assertState(t, "x11", "/old.mp4")
	})
}

func TestServiceSetClearsPreviousBackend(t *testing.T) {
	t.Run("session changed", func(t *testing.T) {
		fx := newServiceFixture(x11Bare)
		fx.state.Set(wallpaper.StateBackend, "wayland")
		v := backendtest.Video(t)

		if b, err := fx.svc.Set(v); err != nil || b != fx.x11 {
			t.Fatalf("Set() = %v, %v", b, err)
		}
		if got, want := fx.mutations(), []string{"wayland.clear", "x11.set"}; !slices.Equal(got, want) {
			t.Errorf("calls = %v, want %v", got, want)
		}
		fx.assertState(t, "x11", v.Path())
	})

	t.Run("override", func(t *testing.T) {
		fx := newServiceFixture(x11Bare)
		fx.state.Set(wallpaper.StateBackend, "x11")
		fx.svc.Override = "WAYLAND"

		if b, err := fx.svc.Set(backendtest.Video(t)); err != nil || b != fx.wayland {
			t.Fatalf("Set() = %v, %v", b, err)
		}
		if got, want := fx.mutations(), []string{"x11.clear", "wayland.set"}; !slices.Equal(got, want) {
			t.Errorf("calls = %v, want %v", got, want)
		}
		if got := fx.state.Get(wallpaper.StateBackend); got != "wayland" {
			t.Errorf("state backend = %q", got)
		}
	})

	t.Run("previous clear failing does not stop set", func(t *testing.T) {
		fx := newServiceFixture(x11Bare)
		fx.state.Set(wallpaper.StateBackend, "wayland")
		fx.wayland.clearErr = errors.New("dbus unreachable")

		if b, err := fx.svc.Set(backendtest.Video(t)); err != nil || b != fx.x11 {
			t.Fatalf("Set() = %v, %v", b, err)
		}
		if !slices.ContainsFunc(*fx.logs, func(l string) bool {
			return strings.Contains(l, "wayland") && strings.Contains(l, "dbus unreachable")
		}) {
			t.Errorf("logs = %q, want the failed clear reported", *fx.logs)
		}
		if got := fx.state.Get(wallpaper.StateBackend); got != "x11" {
			t.Errorf("state backend = %q", got)
		}

		fx.svc.Logf = nil // logging is optional
		fx.state.Set(wallpaper.StateBackend, "wayland")
		if _, err := fx.svc.Set(backendtest.Video(t)); err != nil {
			t.Errorf("Set() without Logf = %v", err)
		}
	})
}

func TestServiceSetSameBackendDoesNotClear(t *testing.T) {
	fx := newServiceFixture(x11Bare)
	fx.state.Set(wallpaper.StateBackend, "x11")

	if _, err := fx.svc.Set(backendtest.Video(t)); err != nil {
		t.Fatal(err)
	}
	if got := fx.mutations(); !slices.Equal(got, []string{"x11.set"}) {
		t.Errorf("calls = %v, want only x11.set", got)
	}
}

func TestServiceSetIgnoresUnknownPreviousBackend(t *testing.T) {
	fx := newServiceFixture(x11Bare)
	fx.state.Set(wallpaper.StateBackend, "uninstalled-plugin")

	if b, err := fx.svc.Set(backendtest.Video(t)); err != nil || b != fx.x11 {
		t.Fatalf("Set() = %v, %v", b, err)
	}
	if got := fx.mutations(); !slices.Equal(got, []string{"x11.set"}) {
		t.Errorf("calls = %v", got)
	}
}

func TestServiceSetFailure(t *testing.T) {
	fx := newServiceFixture(x11Bare)
	fx.state.Set(wallpaper.StateBackend, "x11")
	fx.state.Set(wallpaper.StateVideo, "/old.mp4")
	boom := errors.New("xwinwrap died")
	fx.x11.setErr = boom

	b, err := fx.svc.Set(backendtest.Video(t))
	if b != fx.x11 || !errors.Is(err, boom) || !strings.Contains(err.Error(), "x11") {
		t.Fatalf("Set() = %v, %v; want x11 and its error", b, err)
	}
	if got := fx.state.Get(wallpaper.StateVideo); got != "/old.mp4" {
		t.Errorf("state video = %q, want the previous one kept", got)
	}
}

// A backend may fail after changing the desktop (e.g. one monitor started, the
// next failed). The previous backend is already cleared by then, so the
// failing one must be what a later Clear targets.
func TestServiceSetFailureRecordsFailingBackend(t *testing.T) {
	fx := newServiceFixture(x11Bare)
	fx.state.Set(wallpaper.StateBackend, "wayland")
	fx.x11.setErr = errors.New("second monitor failed")

	if _, err := fx.svc.Set(backendtest.Video(t)); err == nil {
		t.Fatal("Set() = nil error")
	}
	if b, err := fx.svc.Clear(); err != nil || b != fx.x11 {
		t.Errorf("Clear() after failed Set = %v, %v; want x11", b, err)
	}
}

func TestServiceClear(t *testing.T) {
	t.Run("uses the recorded backend over the selected one", func(t *testing.T) {
		fx := newServiceFixture(x11Bare)
		fx.state.Set(wallpaper.StateBackend, "wayland")
		fx.state.Set(wallpaper.StateVideo, "/v.mp4")

		b, err := fx.svc.Clear()
		if err != nil || b != fx.wayland {
			t.Fatalf("Clear() = %v, %v; want wayland", b, err)
		}
		if got := fx.mutations(); !slices.Equal(got, []string{"wayland.clear"}) {
			t.Errorf("calls = %v", got)
		}
		fx.assertState(t, "", "/v.mp4")
	})

	t.Run("recorded name is case-insensitive", func(t *testing.T) {
		fx := newServiceFixture(x11Bare)
		fx.state.Set(wallpaper.StateBackend, "Wayland")
		if b, err := fx.svc.Clear(); err != nil || b != fx.wayland {
			t.Fatalf("Clear() = %v, %v; want wayland", b, err)
		}
	})

	t.Run("falls back to selection when nothing is recorded", func(t *testing.T) {
		fx := newServiceFixture(waylandKDE)
		if b, err := fx.svc.Clear(); err != nil || b != fx.wayland {
			t.Fatalf("Clear() = %v, %v; want wayland", b, err)
		}
		if got := fx.mutations(); !slices.Equal(got, []string{"wayland.clear"}) {
			t.Errorf("calls = %v", got)
		}
	})

	t.Run("falls back to the override", func(t *testing.T) {
		fx := newServiceFixture(waylandKDE)
		fx.svc.Override = "x11"
		if b, err := fx.svc.Clear(); err != nil || b != fx.x11 {
			t.Fatalf("Clear() = %v, %v; want x11", b, err)
		}
	})

	t.Run("falls back to selection when the recorded backend is unknown", func(t *testing.T) {
		fx := newServiceFixture(x11Bare)
		fx.state.Set(wallpaper.StateBackend, "uninstalled-plugin")
		if b, err := fx.svc.Clear(); err != nil || b != fx.x11 {
			t.Fatalf("Clear() = %v, %v; want x11", b, err)
		}
		fx.assertState(t, "", "")
	})

	t.Run("a candidate that is not ready still clears", func(t *testing.T) {
		fx := newServiceFixture(x11Bare)
		fx.x11.checkErr = &wallpaper.MissingDepsError{Deps: []string{"xwinwrap"}}
		if b, err := fx.svc.Clear(); err != nil || b != fx.x11 {
			t.Fatalf("Clear() = %v, %v; want x11", b, err)
		}
		if fx.x11.clears != 1 {
			t.Errorf("x11 clears = %d, want 1", fx.x11.clears)
		}
	})

	// Nothing recorded and nothing fits the session: lazywal cannot have set
	// anything, so there is nothing to clear (e.g. `lazywal clear` from a tty).
	t.Run("nothing recorded and no backend for the session", func(t *testing.T) {
		fx := newServiceFixture(wallpaper.Session{})
		if b, err := fx.svc.Clear(); b != nil || err != nil {
			t.Fatalf("Clear() = %v, %v; want nothing to clear", b, err)
		}
		if len(*fx.journal) != 0 {
			t.Errorf("calls = %v", *fx.journal)
		}
	})

	t.Run("failure keeps the record for a retry", func(t *testing.T) {
		fx := newServiceFixture(x11Bare)
		fx.state.Set(wallpaper.StateBackend, "x11")
		boom := errors.New("dbus unreachable")
		fx.x11.clearErr = boom

		b, err := fx.svc.Clear()
		if b != fx.x11 || !errors.Is(err, boom) || !strings.Contains(err.Error(), "x11") {
			t.Fatalf("Clear() = %v, %v; want x11 and its error", b, err)
		}
		if got := fx.state.Get(wallpaper.StateBackend); got != "x11" {
			t.Errorf("state backend = %q, want x11 kept", got)
		}
	})
}

func TestServiceBackend(t *testing.T) {
	fx := newServiceFixture(waylandKDE)
	if b, err := fx.svc.Backend(); err != nil || b != fx.wayland {
		t.Errorf("Backend() = %v, %v; want wayland", b, err)
	}
	fx.svc.Override = "x11"
	if b, err := fx.svc.Backend(); err != nil || b != fx.x11 {
		t.Errorf("Backend() with override = %v, %v; want x11", b, err)
	}
}

func TestServiceCurrent(t *testing.T) {
	fx := newServiceFixture(x11Bare)
	if v, err := fx.svc.Current(); err == nil || !v.IsZero() {
		t.Fatalf("Current() before Set = %q, %v; want error", v, err)
	}

	v := backendtest.Video(t)
	if _, err := fx.svc.Set(v); err != nil {
		t.Fatal(err)
	}
	if got, err := fx.svc.Current(); err != nil || got != v {
		t.Errorf("Current() = %q, %v; want %q", got, err, v)
	}

	// Clearing stops playback but remembers the video (e.g. for pywal).
	if _, err := fx.svc.Clear(); err != nil {
		t.Fatal(err)
	}
	if got, err := fx.svc.Current(); err != nil || got != v {
		t.Errorf("Current() after Clear = %q, %v; want %q", got, err, v)
	}

	if err := os.Remove(v.Path()); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.svc.Current(); err == nil {
		t.Error("Current() of a deleted file = nil error")
	}
}

// A backends.json entry can replace a built-in under the same name. What the
// built-in started before the override must still be stopped.
func TestServiceClearsShadowedBackend(t *testing.T) {
	journal := &[]string{}
	builtin := &fakeBackend{name: "x11", journal: journal}
	custom := &fakeBackend{name: "x11", journal: journal}
	r := wallpaper.NewRegistry()
	r.Register(builtin, wallpaper.Rule{Session: wallpaper.X11})
	r.Register(custom, wallpaper.Rule{Session: wallpaper.X11})
	state := shelltest.NewStore()
	state.Set(wallpaper.StateBackend, "x11") // set by the built-in, before the override
	svc := &wallpaper.Service{Registry: r, State: state, Session: x11Bare}

	if _, err := svc.Set(backendtest.Video(t)); err != nil {
		t.Fatal(err)
	}
	if builtin.clears != 1 || len(custom.sets) != 1 {
		t.Fatalf("builtin.clears=%d custom.sets=%d; want the shadowed built-in cleared before the override plays", builtin.clears, len(custom.sets))
	}
	if _, err := svc.Clear(); err != nil {
		t.Fatal(err)
	}
	if builtin.clears != 2 || custom.clears != 1 {
		t.Fatalf("builtin.clears=%d custom.clears=%d; want both cleared", builtin.clears, custom.clears)
	}
}

// A cron job inside a desktop environment usually exports only DISPLAY. It
// must keep the backend that set the wallpaper instead of switching to
// whatever a bare X11 session would pick.
func TestServiceInferredSessionKeepsRecordedBackend(t *testing.T) {
	journal := &[]string{}
	bare := &fakeBackend{name: "xwinwrap", journal: journal}
	desktop := &fakeBackend{name: "xfce", journal: journal}
	wl := &fakeBackend{name: "mpvpaper", journal: journal}
	r := wallpaper.NewRegistry()
	r.Register(bare, wallpaper.Rule{Session: wallpaper.X11})
	r.Register(desktop, wallpaper.Rule{Session: wallpaper.X11, Desktops: []string{"xfce"}, Priority: 80})
	r.Register(wl, wallpaper.Rule{Session: wallpaper.Wayland})
	state := shelltest.NewStore()
	cron := wallpaper.Session{Type: wallpaper.X11, Inferred: true}
	svc := &wallpaper.Service{Registry: r, State: state, Session: cron}

	state.Set(wallpaper.StateBackend, "xfce")
	if b, err := svc.Backend(); err != nil || b != desktop {
		t.Fatalf("Backend() = %v, %v; want the recorded xfce backend", b, err)
	}

	// A declared session decides for itself.
	svc.Session = wallpaper.Session{Type: wallpaper.X11}
	if b, _ := svc.Backend(); b != bare {
		t.Fatalf("declared session: Backend() = %v, want xwinwrap", b)
	}

	// A recorded backend for another session type is not reused.
	svc.Session = cron
	state.Set(wallpaper.StateBackend, "mpvpaper")
	if b, _ := svc.Backend(); b != bare {
		t.Fatalf("recorded wayland backend on x11: Backend() = %v, want xwinwrap", b)
	}

	// Neither is one that is not ready any more.
	state.Set(wallpaper.StateBackend, "xfce")
	desktop.checkErr = errors.New("gone")
	if b, _ := svc.Backend(); b != bare {
		t.Fatalf("recorded backend not ready: Backend() = %v, want xwinwrap", b)
	}
}

func TestServiceSetWarnsAboutRemovedPreviousBackend(t *testing.T) {
	fx := newServiceFixture(x11Bare)
	fx.state.Set(wallpaper.StateBackend, "removed-custom")
	if _, err := fx.svc.Set(backendtest.Video(t)); err != nil {
		t.Fatal(err)
	}
	if len(*fx.logs) == 0 || !strings.Contains(strings.Join(*fx.logs, "\n"), "removed-custom is no longer available") {
		t.Errorf("logs = %q, want a warning naming the removed backend", *fx.logs)
	}
}

func TestServiceClearWithUnavailableRecordedBackend(t *testing.T) {
	fx := newServiceFixture(wallpaper.Session{})
	fx.state.Set(wallpaper.StateBackend, "removed-custom")
	if _, err := fx.svc.Clear(); err == nil || !strings.Contains(err.Error(), "removed-custom") {
		t.Fatalf("Clear() error = %v, want it to name the unavailable backend", err)
	}
}
