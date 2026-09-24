package x11_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/backend/x11"
	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

// testSession stands for the login session the tests run in, so no test
// reads the real boot ID or environment.
const testSession = "boot-1/2/:0"

// inSession pins l to the session *current points to, which a test may
// change to simulate a new login.
func inSession(l x11.ProcessLayer, current *string) x11.ProcessLayer {
	l.Session = func() string { return *current }
	return l
}

func fixedSession(l x11.ProcessLayer) x11.ProcessLayer {
	s := testSession
	return inSession(l, &s)
}

// pgrepList is the command a ProcessLayer runs to find its desktop process.
func pgrepList(program string) string { return shell.PgrepList(program).String() }

// shownDesktop scripts fake so each DE shows its desktop the way its session
// starts it.
func shownDesktop(fake *shelltest.Fake) {
	fake.Outputs["gsettings"] = "true"
	fake.Outputs[pgrepList("xfdesktop")] = "1200 xfdesktop --display :0.0 --sm-client-id 2abc"
	fake.Outputs[pgrepList("pcmanfm-qt")] = "1300 pcmanfm-qt --desktop --profile=lxqt"
	fake.Outputs[pgrepList("pcmanfm")] = "1400 pcmanfm --desktop --profile LXDE"
}

var layers = []struct {
	name  string
	layer func(shell.Runner) x11.Layer
}{
	{"mate", func(r shell.Runner) x11.Layer { return x11.MateLayer(r) }},
	{"cinnamon", func(r shell.Runner) x11.Layer { return x11.CinnamonLayer(r) }},
	{"xfce", func(r shell.Runner) x11.Layer { return fixedSession(x11.XfceLayer(r)) }},
	{"lxqt", func(r shell.Runner) x11.Layer { return fixedSession(x11.LXQtLayer(r)) }},
	{"lxde", func(r shell.Runner) x11.Layer { return fixedSession(x11.LXDELayer(r)) }},
}

func newDesktop(fake *shelltest.Fake, store shell.Store, name string, layer x11.Layer, log *logRecorder) *x11.Desktop {
	inner := newXwinwrap(fake, store, x11.DesktopWindow, log)
	var logf func(string, ...any)
	if log != nil {
		logf = log.logf
	}
	return x11.NewDesktop(inner, name, name+" (X11) via xwinwrap", layer, store, logf)
}

func TestDesktopContract(t *testing.T) {
	for _, l := range layers {
		t.Run(l.name, func(t *testing.T) {
			backendtest.ContractWithRunner(t, func(t *testing.T, fake *shelltest.Fake, s *shelltest.Store) wallpaper.Backend {
				fake.Outputs[shell.XrandrCurrent().String()] = twoMonitors
				shownDesktop(fake)
				return newDesktop(fake, s, l.name, l.layer(fake), nil)
			})
		})
	}
	t.Run("any inner backend", func(t *testing.T) {
		backendtest.ContractWithRunner(t, func(t *testing.T, fake *shelltest.Fake, s *shelltest.Store) wallpaper.Backend {
			shownDesktop(fake)
			return x11.NewDesktop(&stubBackend{}, "stub-desktop", "stub", fixedSession(x11.XfceLayer(fake)), s, nil)
		})
	})
}

func TestDesktopCheckListsEveryMissingProgram(t *testing.T) {
	fake := newFake()
	fake.Missing = map[string]bool{"xwinwrap": true, "mpv": true, "xrandr": true}

	err := newDesktop(fake, shelltest.NewStore(), "xfce", fixedSession(x11.XfceLayer(fake)), nil).Check()
	backendtest.MissingDeps(t, err, "xwinwrap", "mpv", "xrandr")
}

// GSettings values are per user, not per session: every Set hides the
// layer again (a new login may show it), but the value to restore stays the
// one captured first, not the Off value later Sets read back.
func TestDesktopGSettingsKeepsFirstValue(t *testing.T) {
	tests := []struct {
		name  string
		layer func(shell.Runner) x11.GSettingsLayer
		get   string // command capturing the current value
		prev  string // value before lazywal
		hide  []string
	}{
		{
			name:  "mate",
			layer: x11.MateLayer,
			get:   "gsettings get org.mate.background show-desktop-icons",
			prev:  "true",
			hide:  []string{"gsettings", "set", "org.mate.background", "show-desktop-icons", "false"},
		},
		{
			name:  "cinnamon",
			layer: x11.CinnamonLayer,
			get:   "gsettings get org.nemo.desktop desktop-layout",
			prev:  "'primary::'",
			hide:  []string{"gsettings", "set", "org.nemo.desktop", "desktop-layout", "'false::false'"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFake()
			fake.Outputs[tt.get] = tt.prev
			store := shelltest.NewStore()
			b := newDesktop(fake, store, tt.name, tt.layer(fake), nil)
			v := backendtest.Video(t)

			if err := b.Set(v); err != nil {
				t.Fatalf("Set() = %v", err)
			}
			fake.Outputs[tt.get] = tt.hide[4] // gsettings now reports the hidden value
			if err := b.Set(v); err != nil {
				t.Fatalf("second Set() = %v", err)
			}
			hide := shell.Command(tt.hide[0], tt.hide[1:]...)
			if n := fake.Count(hide.String()); n != 2 {
				t.Errorf("hid %d times across two Sets, want 2: %q", n, fake.Commands())
			}
			if got := store.Get(x11.DesktopLayerKeyPrefix + tt.name); got != tt.prev {
				t.Errorf("saved state = %q, want the first value %q", got, tt.prev)
			}

			fake.Reset()
			if err := b.Clear(); err != nil {
				t.Fatalf("Clear() = %v", err)
			}
			restore := append(append([]string(nil), tt.hide[:4]...), tt.prev)
			if !ranArgv(fake, restore) {
				t.Errorf("calls = %q, want argv %q", fake.Commands(), restore)
			}
			if got := store.Get(x11.DesktopLayerKeyPrefix + tt.name); got != "" {
				t.Errorf("saved state after Clear = %q", got)
			}

			fake.Reset()
			if err := b.Clear(); err != nil {
				t.Fatalf("second Clear() = %v", err)
			}
			if ranArgv(fake, restore) {
				t.Error("second Clear restored again")
			}
		})
	}
}

func TestDesktopProcessRestoresCommandLine(t *testing.T) {
	tests := []struct {
		name    string
		layer   func(shell.Runner) x11.ProcessLayer
		program string
		running string   // pgrep -a output before lazywal
		off     []string // turns the desktop off
		want    []string // restarted on Clear
		quits   bool     // off stops the process
	}{
		{
			name: "xfce", layer: x11.XfceLayer, program: "xfdesktop",
			running: "1200 xfdesktop --display :0.0 --sm-client-id 2abc",
			off:     []string{"xfdesktop", "--quit"},
			want:    []string{"xfdesktop", "--display", ":0.0", "--sm-client-id", "2abc"},
			quits:   true,
		},
		{
			name: "lxqt", layer: x11.LXQtLayer, program: "pcmanfm-qt",
			running: "1300 pcmanfm-qt --desktop --profile=lxqt",
			off:     []string{"pcmanfm-qt", "--desktop-off"},
			want:    []string{"pcmanfm-qt", "--desktop", "--profile=lxqt"},
		},
		{
			name: "lxde keeps its profile", layer: x11.LXDELayer, program: "pcmanfm",
			running: "1400 pcmanfm --desktop --profile LXDE",
			off:     []string{"pcmanfm", "--desktop-off"},
			want:    []string{"pcmanfm", "--desktop", "--profile", "LXDE"},
		},
		{
			name: "lxde desktop instance among file managers", layer: x11.LXDELayer, program: "pcmanfm",
			running: "1398 pcmanfm /home/u\n1400 /usr/bin/pcmanfm --desktop --profile LXDE\n",
			off:     []string{"pcmanfm", "--desktop-off"},
			want:    []string{"/usr/bin/pcmanfm", "--desktop", "--profile", "LXDE"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFake()
			fake.Outputs[pgrepList(tt.program)] = tt.running
			store := shelltest.NewStore()
			key := x11.DesktopLayerKeyPrefix + "de"
			b := newDesktop(fake, store, "de", fixedSession(tt.layer(fake)), nil)
			v := backendtest.Video(t)

			if err := b.Set(v); err != nil {
				t.Fatalf("Set() = %v", err)
			}
			if !ranArgv(fake, tt.off) {
				t.Errorf("calls = %q, want %q", fake.Commands(), tt.off)
			}
			saved := store.Get(key)
			if got := savedArgv(t, saved, testSession); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("saved argv %q, want %q", got, tt.want)
			}

			if tt.quits {
				pgrep := shell.PgrepList(tt.program)
				fake.Errors[pgrep.String()] = shelltest.Exit(pgrep, 1)
			}
			if err := b.Set(v); err != nil {
				t.Fatalf("second Set() = %v", err)
			}
			if got := store.Get(key); got != saved {
				t.Errorf("second Set replaced the saved state %q with %q", saved, got)
			}

			fake.Reset()
			if err := b.Clear(); err != nil {
				t.Fatalf("Clear() = %v", err)
			}
			if len(fake.Started) != 1 || !reflect.DeepEqual(fake.Started[0].Argv(), tt.want) {
				t.Errorf("started %q, want only %q", fake.Started, tt.want)
			}
			if pids := trackedPIDs(store); len(pids) != 0 {
				t.Errorf("restarted desktop is tracked (%v); the next Set would kill it", pids)
			}
			if got := store.Get(key); got != "" {
				t.Errorf("saved state after Clear = %q", got)
			}
		})
	}
}

// savedArgv decodes a ProcessLayer state, checking it names session.
func savedArgv(t *testing.T, state, session string) []string {
	t.Helper()
	var s struct {
		Session string   `json:"session"`
		Argv    []string `json:"argv"`
	}
	if err := json.Unmarshal([]byte(state), &s); err != nil {
		t.Fatalf("saved state %q: %v", state, err)
	}
	if s.Session != session {
		t.Errorf("saved state %q is for session %q, want %q", state, s.Session, session)
	}
	return s.Argv
}

// A desktop that is not shown is neither turned off nor restarted.
func TestDesktopProcessNotShown(t *testing.T) {
	tests := []struct {
		name    string
		layer   func(shell.Runner) x11.ProcessLayer
		program string
		running string // pgrep -a output; "" means pgrep exits 1
	}{
		{"xfdesktop not running", x11.XfceLayer, "xfdesktop", ""},
		{"pcmanfm not running", x11.LXDELayer, "pcmanfm", ""},
		{"pcmanfm without --desktop", x11.LXDELayer, "pcmanfm", "1400 pcmanfm --profile LXDE"},
		{"pcmanfm-qt daemon only", x11.LXQtLayer, "pcmanfm-qt", "1300 pcmanfm-qt --daemon-mode"},
		{"pcmanfm with --desktop-off", x11.LXDELayer, "pcmanfm", "1400 pcmanfm --desktop-off"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFake()
			pgrep := shell.PgrepList(tt.program)
			if tt.running == "" {
				fake.Errors[pgrep.String()] = shelltest.Exit(pgrep, 1)
			} else {
				fake.Outputs[pgrep.String()] = tt.running
			}
			store := shelltest.NewStore()
			key := x11.DesktopLayerKeyPrefix + "de"
			b := newDesktop(fake, store, "de", fixedSession(tt.layer(fake)), nil)

			if err := b.Set(backendtest.Video(t)); err != nil {
				t.Fatalf("Set() = %v", err)
			}
			for _, c := range fake.Calls {
				if c.Name == tt.program {
					t.Errorf("ran %q although the desktop was not shown", c)
				}
			}
			if got := store.Get(key); got != "not-shown" {
				t.Errorf("saved state = %q, want not-shown", got)
			}

			fake.Reset()
			if err := b.Clear(); err != nil {
				t.Fatalf("Clear() = %v", err)
			}
			if len(fake.Started) != 0 {
				t.Errorf("Clear started %q, want nothing", fake.Started)
			}
			if got := store.Get(key); got != "" {
				t.Errorf("saved state after Clear = %q", got)
			}
		})
	}
}

// After a new login the saved state belongs to the old session: Set must
// hide the new session's desktop, and Clear must restart that one.
func TestDesktopReLogin(t *testing.T) {
	fake := newFake()
	fake.Outputs[pgrepList("pcmanfm")] = "1400 pcmanfm --desktop --profile LXDE"
	store := shelltest.NewStore()
	key := x11.DesktopLayerKeyPrefix + "lxde"
	session := "boot-1/2/:0"
	b := newDesktop(fake, store, "lxde", inSession(x11.LXDELayer(fake), &session), nil)
	v := backendtest.Video(t)

	if err := b.Set(v); err != nil {
		t.Fatalf("Set() = %v", err)
	}

	session = "boot-2/5/:1"
	fake.Outputs[pgrepList("pcmanfm")] = "900 pcmanfm --desktop --profile LXDE-pi"
	fake.Reset()
	if err := b.Set(v); err != nil {
		t.Fatalf("Set() after re-login = %v", err)
	}
	if !fake.Ran("pcmanfm --desktop-off") {
		t.Errorf("calls = %q, want the new session's desktop hidden", fake.Commands())
	}
	want := []string{"pcmanfm", "--desktop", "--profile", "LXDE-pi"}
	if got := savedArgv(t, store.Get(key), session); !reflect.DeepEqual(got, want) {
		t.Errorf("saved argv %q, want the new session's %q", got, want)
	}

	fake.Reset()
	if err := b.Clear(); err != nil {
		t.Fatalf("Clear() = %v", err)
	}
	if len(fake.Started) != 1 || !reflect.DeepEqual(fake.Started[0].Argv(), want) {
		t.Errorf("started %q, want only %q", fake.Started, want)
	}
}

// A Clear in another session must not restart a desktop captured in the
// old one (it would run in the wrong session); the stale state is dropped.
func TestDesktopClearInAnotherSession(t *testing.T) {
	fake := newFake()
	fake.Outputs[pgrepList("xfdesktop")] = "1200 xfdesktop"
	store := shelltest.NewStore()
	key := x11.DesktopLayerKeyPrefix + "xfce"
	session := "boot-1/2/:0"
	b := newDesktop(fake, store, "xfce", inSession(x11.XfceLayer(fake), &session), nil)

	if err := b.Set(backendtest.Video(t)); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	if store.Get(key) == "" {
		t.Fatal("nothing saved")
	}

	session = "boot-1/3/:0"
	fake.Reset()
	if err := b.Clear(); err != nil {
		t.Fatalf("Clear() = %v", err)
	}
	if len(fake.Started) != 0 {
		t.Errorf("Clear started %q in another session", fake.Started)
	}
	if got := store.Get(key); got != "" {
		t.Errorf("saved state after Clear = %q, want the stale one dropped", got)
	}
}

// Nothing is lost by replacing not-shown, so a desktop shown since (e.g.
// started by hand) is hidden and later restored.
func TestDesktopNotShownIsReplaced(t *testing.T) {
	fake := newFake()
	pgrep := shell.PgrepList("xfdesktop")
	fake.Errors[pgrep.String()] = shelltest.Exit(pgrep, 1)
	store := shelltest.NewStore()
	key := x11.DesktopLayerKeyPrefix + "xfce"
	b := newDesktop(fake, store, "xfce", fixedSession(x11.XfceLayer(fake)), nil)
	v := backendtest.Video(t)

	if err := b.Set(v); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	delete(fake.Errors, pgrep.String())
	fake.Outputs[pgrep.String()] = "1200 xfdesktop"
	if err := b.Set(v); err != nil {
		t.Fatalf("second Set() = %v", err)
	}
	if got := savedArgv(t, store.Get(key), testSession); !reflect.DeepEqual(got, []string{"xfdesktop"}) {
		t.Errorf("saved argv %q, want [xfdesktop]", got)
	}
}

func TestDesktopHideFailureStillPlays(t *testing.T) {
	pgrep := shell.PgrepList("pcmanfm")
	tests := []struct {
		name   string
		layer  func(shell.Runner) x11.Layer
		script func(*shelltest.Fake)
	}{
		{"gsettings get fails", func(r shell.Runner) x11.Layer { return x11.MateLayer(r) }, func(f *shelltest.Fake) {
			f.Errors["gsettings get org.mate.background show-desktop-icons"] = errors.New("no such schema")
		}},
		{"pgrep fails", func(r shell.Runner) x11.Layer { return fixedSession(x11.LXDELayer(r)) }, func(f *shelltest.Fake) {
			f.Errors[pgrep.String()] = shelltest.Exit(pgrep, 3)
		}},
		{"desktop-off fails", func(r shell.Runner) x11.Layer { return fixedSession(x11.LXDELayer(r)) }, func(f *shelltest.Fake) {
			f.Outputs[pgrep.String()] = "1400 pcmanfm --desktop --profile LXDE"
			f.Errors["pcmanfm --desktop-off"] = errors.New("no instance")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFake()
			tt.script(fake)
			store := shelltest.NewStore()
			log := &logRecorder{}
			b := newDesktop(fake, store, "de", tt.layer(fake), log)

			if err := b.Set(backendtest.Video(t)); err != nil {
				t.Fatalf("Set() = %v, want the video to play anyway", err)
			}
			if len(fake.Started) != 2 {
				t.Errorf("started %d players, want 2", len(fake.Started))
			}
			if !log.contains("may be covered") {
				t.Errorf("log = %q, want a warning", log.lines)
			}
			if got := store.Get(x11.DesktopLayerKeyPrefix + "de"); got != "" {
				t.Errorf("saved state = %q after a failed hide", got)
			}

			fake.Reset()
			if err := b.Clear(); err != nil {
				t.Fatalf("Clear() = %v", err)
			}
			for _, c := range fake.Calls {
				if c.Name == "gsettings" || strings.HasPrefix(c.Name, "pcmanfm") {
					t.Errorf("Clear restored a layer it never hid: %q", c)
				}
			}
		})
	}
}

func TestDesktopEmptyGSettingsValueIsNotSaved(t *testing.T) {
	fake := newFake() // gsettings get prints nothing
	store := shelltest.NewStore()
	b := newDesktop(fake, store, "mate", x11.MateLayer(fake), &logRecorder{})

	if err := b.Set(backendtest.Video(t)); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	if fake.Ran("gsettings set org.mate.background show-desktop-icons false") {
		t.Error("hid the layer without a value to restore")
	}
	if got := store.Get("lazywal_desktop_layer_mate"); got != "" {
		t.Errorf("saved state = %q", got)
	}
}

// stubBackend records calls, standing in for any wallpaper.Backend.
type stubBackend struct {
	sets, clears     int
	setErr, clearErr error
}

func (*stubBackend) Name() string     { return "stub" }
func (*stubBackend) Describe() string { return "stub backend" }
func (*stubBackend) Check() error     { return nil }

func (s *stubBackend) Set(wallpaper.Video) error {
	s.sets++
	return s.setErr
}

func (s *stubBackend) Clear() error {
	s.clears++
	return s.clearErr
}

func TestDesktopDecoratesAnyBackend(t *testing.T) {
	fake := shelltest.New()
	fake.Outputs["gsettings"] = "true"
	store := shelltest.NewStore()
	inner := &stubBackend{setErr: errors.New("set failed"), clearErr: errors.New("clear failed")}
	fake.Errors["gsettings set org.mate.background show-desktop-icons true"] = errors.New("restore failed")
	b := x11.NewDesktop(inner, "mate", "MATE", x11.MateLayer(fake), store, nil)

	if b.Name() != "mate" || b.Describe() != "MATE" {
		t.Errorf("identity = %q, %q; want the decorator's own", b.Name(), b.Describe())
	}
	if err := b.Set(backendtest.Video(t)); !errors.Is(err, inner.setErr) || inner.sets != 1 {
		t.Errorf("Set() = %v (inner sets %d), want the inner error", err, inner.sets)
	}
	if err := b.Clear(); !errors.Is(err, inner.clearErr) || inner.clears != 1 {
		t.Errorf("Clear() = %v (inner clears %d), want the inner error first", err, inner.clears)
	}
	if !fake.Ran("gsettings set org.mate.background show-desktop-icons true") {
		t.Error("layer not restored when the inner Clear failed")
	}
	if got := store.Get("lazywal_desktop_layer_mate"); got != "" {
		t.Errorf("saved state after Clear = %q", got)
	}
}

func TestDesktopRestoreFailureIsReported(t *testing.T) {
	fake := shelltest.New()
	fake.Outputs[pgrepList("pcmanfm-qt")] = "1300 pcmanfm-qt --desktop --profile=lxqt"
	fake.Errors["pcmanfm-qt --desktop --profile=lxqt"] = errors.New("cannot start")
	store := shelltest.NewStore()
	b := x11.NewDesktop(&stubBackend{}, "lxqt", "LXQt", fixedSession(x11.LXQtLayer(fake)), store, nil)

	if err := b.Set(backendtest.Video(t)); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	if err := b.Clear(); err == nil {
		t.Fatal("Clear() = nil, want the restore error")
	}
	if got := store.Get("lazywal_desktop_layer_lxqt"); got != "" {
		t.Errorf("saved state = %q, want it dropped so Clear can be repeated", got)
	}
	if err := b.Clear(); err != nil {
		t.Errorf("second Clear() = %v", err)
	}
}

// ranArgv reports whether a call had exactly argv.
func ranArgv(fake *shelltest.Fake, argv []string) bool {
	for _, c := range fake.Calls {
		if reflect.DeepEqual(c.Argv(), argv) {
			return true
		}
	}
	return false
}
