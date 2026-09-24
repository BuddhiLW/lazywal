package custom

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

func newBackend(t *testing.T, spec Spec) (*Backend, *shelltest.Fake, *shelltest.Store) {
	t.Helper()
	r, st := shelltest.New(), shelltest.NewStore()
	b, err := New(spec, r, st)
	if err != nil {
		t.Fatal(err)
	}
	return b, r, st
}

func TestContract(t *testing.T) {
	specs, err := Load(example)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range append(specs, valid()) {
		t.Run(spec.Name, func(t *testing.T) {
			backendtest.ContractWithRunner(t, func(t *testing.T, r *shelltest.Fake, s *shelltest.Store) wallpaper.Backend {
				b, err := New(spec, r, s)
				if err != nil {
					t.Fatal(err)
				}
				return b
			})
		})
	}
}

func TestNewRejectsInvalidSpec(t *testing.T) {
	s := valid()
	s.Name = "Bad Name"
	if b, err := New(s, shelltest.New(), shelltest.NewStore()); err == nil {
		t.Errorf("New() = %v, want an error", b)
	}
}

func TestNewCopiesSpec(t *testing.T) {
	s := valid()
	b, r, _ := newBackend(t, s)
	s.Set[0][0] = "rm"
	if err := b.Set(backendtest.Video(t)); err != nil {
		t.Fatal(err)
	}
	if got := r.Calls[0].Name; got != "notify-send" {
		t.Errorf("ran %q after the caller edited its spec", got)
	}
}

func TestCheckListsEveryMissingProgramOnce(t *testing.T) {
	spec := Spec{
		Name:        "demo",
		Description: "demo",
		Requires:    []string{"mpv", "xwinwrap", "mpv"},
		Set:         [][]string{{"gsettings", "set", "a", "b", "{{path}}"}, {"gsettings", "set", "c", "d", "e"}},
		Spawn:       [][]string{{"xwinwrap", "--", "mpv", "{{path}}"}},
		Clear:       [][]string{{"gnome-extensions", "disable", "x"}},
	}
	b, r, _ := newBackend(t, spec)
	for _, p := range []string{"mpv", "xwinwrap", "gsettings", "gnome-extensions"} {
		r.Missing[p] = true
	}
	backendtest.MissingDeps(t, b.Check(), "mpv", "xwinwrap", "gsettings", "gnome-extensions")

	r.Missing = map[string]bool{"gnome-extensions": true}
	backendtest.MissingDeps(t, b.Check(), "gnome-extensions")
}

func TestSetExpandsPlaceholdersInsideArguments(t *testing.T) {
	spec := Spec{
		Name:        "demo",
		Description: "demo",
		Set:         [][]string{{"tool", "--file={{path}}", "{{uri}}", "{{path}}|{{uri}}", "plain"}},
		Spawn:       [][]string{{"player", "{{path}}"}},
	}
	b, r, _ := newBackend(t, spec)
	v := backendtest.Video(t) // its name has spaces and quotes
	if err := b.Set(v); err != nil {
		t.Fatal(err)
	}
	want := []string{"tool", "--file=" + v.Path(), v.URI(), v.Path() + "|" + v.URI(), "plain"}
	if got := r.Calls[0].Argv(); !reflect.DeepEqual(got, want) {
		t.Errorf("set argv = %q\nwant       %q", got, want)
	}
	if got, want := r.Started[0].Argv(), []string{"player", v.Path()}; !reflect.DeepEqual(got, want) {
		t.Errorf("spawn argv = %q, want %q", got, want)
	}
}

func TestSetDoesNotExpandPlaceholdersInThePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "{{uri}}.mp4")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := wallpaper.NewVideo(path)
	if err != nil {
		t.Fatal(err)
	}
	b, r, _ := newBackend(t, Spec{Name: "demo", Description: "demo", Spawn: [][]string{{"player", "{{path}}"}}})
	if err := b.Set(v); err != nil {
		t.Fatal(err)
	}
	if got := r.Started[0].Args; !reflect.DeepEqual(got, []string{path}) {
		t.Errorf("args = %q, want %q", got, path)
	}
}

func TestSetRunsCommandsInOrderAndTracksPlayers(t *testing.T) {
	spec := Spec{
		Name:        "demo",
		Description: "demo",
		Set:         [][]string{{"first"}, {"second"}},
		Spawn:       [][]string{{"player-a"}, {"player-b"}},
	}
	b, r, st := newBackend(t, spec)
	v := backendtest.Video(t)
	if err := b.Set(v); err != nil {
		t.Fatal(err)
	}
	if got, want := r.Commands(), []string{"first", "second", "player-a", "player-b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %q, want %q", got, want)
	}
	if got := (shell.Tracker{Store: st, Key: "lazywal_custom_demo_pids"}).PIDs(); !reflect.DeepEqual(got, []int{100001, 100002}) {
		t.Errorf("tracked pids = %v", got)
	}

	r.Reset()
	if err := b.Set(v); err != nil {
		t.Fatal(err)
	}
	if want := []int{100001, 100002}; !reflect.DeepEqual(r.Killed, want) {
		t.Errorf("second Set killed %v, want %v", r.Killed, want)
	}
}

func TestSetStopsAtFirstFailure(t *testing.T) {
	spec := Spec{
		Name:        "demo",
		Description: "demo",
		Set:         [][]string{{"first"}, {"second"}},
		Spawn:       [][]string{{"player"}},
	}
	b, r, _ := newBackend(t, spec)
	boom := errors.New("boom")
	r.Errors["first"] = boom
	if err := b.Set(backendtest.Video(t)); !errors.Is(err, boom) {
		t.Fatalf("Set() = %v, want %v", err, boom)
	}
	if got := r.Commands(); !reflect.DeepEqual(got, []string{"first"}) {
		t.Errorf("calls = %q, want only the failing command", got)
	}
}

func TestClearRunsEveryCommandAndJoinsErrors(t *testing.T) {
	spec := Spec{
		Name:        "demo",
		Description: "demo",
		Spawn:       [][]string{{"player", "{{path}}"}},
		Clear:       [][]string{{"undo-a"}, {"undo-b"}, {"undo-c"}},
	}
	b, r, st := newBackend(t, spec)
	if err := b.Set(backendtest.Video(t)); err != nil {
		t.Fatal(err)
	}
	r.Reset()
	errA, errC := errors.New("a failed"), errors.New("c failed")
	r.Errors["undo-a"], r.Errors["undo-c"] = errA, errC

	err := b.Clear()
	if !errors.Is(err, errA) || !errors.Is(err, errC) {
		t.Errorf("Clear() = %v, want both failures", err)
	}
	if got, want := r.Commands(), []string{"undo-a", "undo-b", "undo-c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(r.Killed, []int{100001}) || st.Get("lazywal_custom_demo_pids") != "" {
		t.Errorf("killed %v, tracked %q; want the player stopped and forgotten", r.Killed, st.Get("lazywal_custom_demo_pids"))
	}
}

// builtin stands in for a backend lazywal ships.
type builtin struct{ name string }

func (b builtin) Name() string            { return b.name }
func (builtin) Describe() string          { return "built-in" }
func (builtin) Check() error              { return nil }
func (builtin) Set(wallpaper.Video) error { return nil }
func (builtin) Clear() error              { return nil }

func names(bs []wallpaper.Backend) []string {
	out := []string{}
	for _, b := range bs {
		out = append(out, b.Name())
	}
	return out
}

func TestRegister(t *testing.T) {
	specs, err := Load(example)
	if err != nil {
		t.Fatal(err)
	}
	newRegistry := func() *wallpaper.Registry {
		reg := wallpaper.NewRegistry()
		reg.Register(builtin{"mpvpaper"}, wallpaper.Rule{Session: wallpaper.Wayland, Priority: 100})
		return reg
	}

	t.Run("higher priority rule wins", func(t *testing.T) {
		reg := newRegistry()
		if err := Register(reg, specs, shelltest.New(), shelltest.NewStore()); err != nil {
			t.Fatal(err)
		}
		hyprland := wallpaper.Session{Type: wallpaper.Wayland, Desktops: []string{"hyprland"}}
		if got, want := names(reg.Candidates(hyprland)), []string{"hyprland-dp1", "mpvpaper"}; !reflect.DeepEqual(got, want) {
			t.Errorf("hyprland candidates = %q, want %q", got, want)
		}
		sway := wallpaper.Session{Type: wallpaper.Wayland, Desktops: []string{"sway"}}
		if got, want := names(reg.Candidates(sway)), []string{"mpvpaper"}; !reflect.DeepEqual(got, want) {
			t.Errorf("sway candidates = %q, want %q", got, want)
		}
	})

	t.Run("spec without rules is only selected by name", func(t *testing.T) {
		reg := newRegistry()
		if err := Register(reg, specs, shelltest.New(), shelltest.NewStore()); err != nil {
			t.Fatal(err)
		}
		openbox := wallpaper.Session{Type: wallpaper.X11, Desktops: []string{"openbox"}}
		if got := names(reg.Candidates(openbox)); len(got) != 0 {
			t.Errorf("openbox candidates = %q, want none", got)
		}
		if b, err := reg.Select(openbox, "xwinwrap-hwdec"); err != nil || b.Name() != "xwinwrap-hwdec" {
			t.Errorf("Select(override) = %v, %v", b, err)
		}
	})

	t.Run("same name replaces a built-in", func(t *testing.T) {
		reg := newRegistry()
		override := Spec{
			Name:        "mpvpaper",
			Description: "my mpvpaper",
			Rules:       []wallpaper.Rule{{Session: wallpaper.Wayland, Desktops: []string{"sway"}}},
			Spawn:       [][]string{{"mpvpaper", "-o", "hwdec=auto", "ALL", "{{path}}"}},
		}
		if err := Register(reg, []Spec{override}, shelltest.New(), shelltest.NewStore()); err != nil {
			t.Fatal(err)
		}
		b, err := reg.Get("mpvpaper")
		if _, ok := b.(*Backend); err != nil || !ok {
			t.Fatalf("Get(mpvpaper) = %T, %v; want the custom backend", b, err)
		}
		river := wallpaper.Session{Type: wallpaper.Wayland, Desktops: []string{"river"}}
		if got := names(reg.Candidates(river)); len(got) != 0 {
			t.Errorf("river candidates = %q, want none: the override's rules replace the built-in's", got)
		}
	})

	t.Run("invalid spec registers nothing", func(t *testing.T) {
		reg := wallpaper.NewRegistry()
		bad := Spec{Name: "bad"}
		if err := Register(reg, []Spec{specs[0], bad}, shelltest.New(), shelltest.NewStore()); err == nil {
			t.Fatal("Register() = nil, want an error")
		}
		if got := names(reg.All()); len(got) != 0 {
			t.Errorf("registered %q despite the error", got)
		}
	})

	t.Run("duplicate names are rejected", func(t *testing.T) {
		if err := Register(wallpaper.NewRegistry(), []Spec{specs[0], specs[0]}, shelltest.New(), shelltest.NewStore()); err == nil {
			t.Fatal("Register() = nil, want an error")
		}
	})
}

// A backend removed from backends.json may still have players running; the
// stand-in stops them but refuses to play.
func TestRetiredStopsWhatTheRemovedBackendStarted(t *testing.T) {
	r, st := shelltest.New(), shelltest.NewStore()
	b, err := New(Spec{Name: "demo", Description: "d", Spawn: [][]string{{"player", "{{path}}"}}}, r, st)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Set(backendtest.Video(t)); err != nil {
		t.Fatal(err)
	}
	if !HasPlayers("demo", st) {
		t.Fatal("HasPlayers = false after Set")
	}

	old := Retired("demo", r, st)
	if err := old.Set(backendtest.Video(t)); err == nil {
		t.Error("retired Set succeeded")
	}
	if err := old.Check(); err == nil {
		t.Error("retired Check passed")
	}
	if err := old.Clear(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Killed, []int{100001}) || HasPlayers("demo", st) {
		t.Errorf("killed %v, still tracked %v", r.Killed, HasPlayers("demo", st))
	}
}
