package x11_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/backend/x11"
	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

// logRecorder collects formatted log lines.
type logRecorder struct{ lines []string }

func (l *logRecorder) logf(format string, args ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *logRecorder) contains(sub string) bool {
	return slices.ContainsFunc(l.lines, func(s string) bool { return strings.Contains(s, sub) })
}

// newFake returns a runner with two monitors attached.
func newFake() *shelltest.Fake {
	fake := shelltest.New()
	fake.Outputs[shell.XrandrCurrent().String()] = twoMonitors
	return fake
}

func newXwinwrap(fake *shelltest.Fake, store shell.Store, mode x11.WindowMode, log *logRecorder) *x11.Xwinwrap {
	cfg := x11.XwinwrapConfig{
		Name:        "xwinwrap",
		Description: "X11 window managers via xwinwrap + mpv",
		Mode:        mode,
		Runner:      fake,
		Store:       store,
	}
	if log != nil {
		cfg.Logf = log.logf
	}
	return x11.NewXwinwrap(cfg)
}

var modes = []struct {
	name string
	mode x11.WindowMode
}{
	{"unmanaged", x11.Unmanaged},
	{"desktop window", x11.DesktopWindow},
}

func TestXwinwrapContract(t *testing.T) {
	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			backendtest.ContractWithRunner(t, func(t *testing.T, fake *shelltest.Fake, s *shelltest.Store) wallpaper.Backend {
				fake.Outputs[shell.XrandrCurrent().String()] = twoMonitors
				return newXwinwrap(fake, s, m.mode, nil)
			})
		})
	}
}

func TestXwinwrapCheckListsEveryMissingProgram(t *testing.T) {
	fake := newFake()
	fake.Missing = map[string]bool{"xwinwrap": true, "mpv": true, "xrandr": true}

	err := newXwinwrap(fake, shelltest.NewStore(), x11.Unmanaged, nil).Check()
	backendtest.MissingDeps(t, err, "xwinwrap", "mpv", "xrandr")
	if len(fake.Calls) != 0 {
		t.Errorf("Check ran commands: %q", fake.Commands())
	}
}

var (
	unmanagedFlags     = []string{"-ni", "-b", "-st", "-un", "-o", "1", "-ov"}
	desktopWindowFlags = []string{"-ni", "-fdt", "-b", "-nf", "-s", "-st", "-sp", "-un", "-o", "1"}
)

// playerArgv is the full xwinwrap + mpv argv expected for one monitor.
func playerArgv(geometry string, flags []string, path string) []string {
	argv := append([]string{"xwinwrap", "-g", geometry}, flags...)
	argv = append(argv, "--", "mpv", "-wid", "WID", "--loop-file=inf", "--no-audio", "--no-resume-playback", "--panscan=1")
	return append(argv, "--", path)
}

func trackedPIDs(store shell.Store) []int {
	return shell.Tracker{Store: store, Key: x11.PIDsKey}.PIDs()
}

func TestXwinwrapSetArgv(t *testing.T) {
	tests := []struct {
		name  string
		mode  x11.WindowMode
		flags []string
	}{
		{"unmanaged", x11.Unmanaged, unmanagedFlags},
		{"desktop window", x11.DesktopWindow, desktopWindowFlags},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFake()
			v := backendtest.Video(t)
			if err := newXwinwrap(fake, shelltest.NewStore(), tt.mode, nil).Set(v); err != nil {
				t.Fatalf("Set() = %v", err)
			}

			want := [][]string{
				playerArgv("1920x1200+0+0", tt.flags, v.Path()),
				playerArgv("2560x1080+1920+0", tt.flags, v.Path()),
			}
			if len(fake.Started) != len(want) {
				t.Fatalf("started %d commands, want %d: %q", len(fake.Started), len(want), fake.Started)
			}
			for i, c := range fake.Started {
				if got := c.Argv(); !reflect.DeepEqual(got, want[i]) {
					t.Errorf("monitor %d argv =\n  %q\nwant\n  %q", i, got, want[i])
				}
				if last := c.Args[len(c.Args)-1]; last != v.Path() || !strings.Contains(last, `it's a "video"`) {
					t.Errorf("video path not passed as one argument: %q", last)
				}
			}
		})
	}
}

func TestXwinwrapSetReplacesTrackedProcesses(t *testing.T) {
	fake := newFake()
	store := shelltest.NewStore()
	// Bare PIDs written by lazywal <= v1.4.3, which ran "bash -c xwinwrap ...".
	store.Set(x11.PIDsKey, "11,22,33,44")
	fake.Names[11] = "xwinwrap"
	fake.Names[22] = "bash"
	fake.Names[33] = "firefox" // PID reused since
	// 44 is gone.
	b := newXwinwrap(fake, store, x11.Unmanaged, nil)
	v := backendtest.Video(t)

	if err := b.Set(v); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	if !reflect.DeepEqual(fake.Killed, []int{11, 22}) {
		t.Errorf("killed %v, want the older version's players only", fake.Killed)
	}
	first := trackedPIDs(store)
	if len(first) != 2 {
		t.Fatalf("tracked PIDs = %v, want one per monitor", first)
	}

	fake.Reset()
	if err := b.Set(v); err != nil {
		t.Fatalf("second Set() = %v", err)
	}
	if !reflect.DeepEqual(fake.Killed, first) {
		t.Errorf("second Set killed %v, want the first Set's PIDs %v", fake.Killed, first)
	}
	if len(fake.Started) != 2 {
		t.Errorf("second Set started %d processes, want 2", len(fake.Started))
	}
}

func TestXwinwrapSetStartsWhatItCan(t *testing.T) {
	fake := newFake()
	store := shelltest.NewStore()
	log := &logRecorder{}
	v := backendtest.Video(t)
	failing := playerArgv("1920x1200+0+0", unmanagedFlags, v.Path())
	fake.Errors[shell.Command(failing[0], failing[1:]...).String()] = errors.New("boom")

	if err := newXwinwrap(fake, store, x11.Unmanaged, log).Set(v); err != nil {
		t.Fatalf("Set() = %v, want success with one monitor started", err)
	}
	if len(fake.Started) != 1 || fake.Started[0].Args[1] != "2560x1080+1920+0" {
		t.Errorf("started %q, want only HDMI-1-0", fake.Started)
	}
	if !log.contains("Failed to start on monitor eDP-1") || !log.contains("Started wallpaper on monitor HDMI-1-0") {
		t.Errorf("log = %q", log.lines)
	}
	if pids := trackedPIDs(store); len(pids) != 1 {
		t.Errorf("tracked PIDs = %v, want one", pids)
	}
}

func TestXwinwrapSetFailsWhenNoMonitorStarts(t *testing.T) {
	fake := newFake()
	boom := errors.New("boom")
	fake.Errors["xwinwrap"] = boom

	err := newXwinwrap(fake, shelltest.NewStore(), x11.Unmanaged, nil).Set(backendtest.Video(t))
	if !errors.Is(err, boom) {
		t.Errorf("Set() = %v, want an error wrapping %v", err, boom)
	}
}

func TestXwinwrapSetKeepsOldVideoWhenMonitorsUnknown(t *testing.T) {
	fake := shelltest.New()
	fake.Errors["xrandr"] = errors.New("cannot open display")
	store := shelltest.NewStore()
	store.Set(x11.PIDsKey, "11")

	if err := newXwinwrap(fake, store, x11.Unmanaged, nil).Set(backendtest.Video(t)); err == nil {
		t.Fatal("Set() = nil, want error")
	}
	if len(fake.Killed) != 0 || store.Get(x11.PIDsKey) != "11" {
		t.Errorf("killed %v (tracked %q) although nothing could replace it", fake.Killed, store.Get(x11.PIDsKey))
	}
}

type staticMonitors []x11.Monitor

func (s staticMonitors) Monitors() ([]x11.Monitor, error) { return s, nil }

func TestXwinwrapUsesGivenMonitorSource(t *testing.T) {
	fake := shelltest.New()
	b := x11.NewXwinwrap(x11.XwinwrapConfig{
		Name: "xwinwrap", Description: "d", Runner: fake, Store: shelltest.NewStore(),
		Monitors: staticMonitors{{Name: "VGA-1", Width: 800, Height: 600, X: 10, Y: 20}},
	})
	if err := b.Set(backendtest.Video(t)); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	if fake.Ran("xrandr --current") {
		t.Error("xrandr ran although a monitor source was given")
	}
	if len(fake.Started) != 1 || fake.Started[0].Args[1] != "800x600+10+20" {
		t.Errorf("started %q", fake.Started)
	}
	if !slices.Contains(fake.Started[0].Args, "-ov") {
		t.Errorf("nil Mode should default to Unmanaged: %q", fake.Started[0])
	}
}

func TestXwinwrapClear(t *testing.T) {
	fake := newFake()
	store := shelltest.NewStore()
	b := newXwinwrap(fake, store, x11.Unmanaged, nil)
	if err := b.Set(backendtest.Video(t)); err != nil {
		t.Fatalf("Set() = %v", err)
	}
	fake.Reset()

	if err := b.Clear(); err != nil {
		t.Fatalf("Clear() = %v", err)
	}
	if len(fake.Killed) != 2 {
		t.Errorf("killed %v, want both players", fake.Killed)
	}
	if !fake.Ran(shell.Pkill("xwinwrap").String()) {
		t.Errorf("calls = %q, want stray xwinwrap killed", fake.Commands())
	}
	if got := store.Get(x11.PIDsKey); got != "" {
		t.Errorf("tracked PIDs after Clear = %q", got)
	}
}

func TestXwinwrapClearPkillStatus(t *testing.T) {
	pkill := shell.Pkill("xwinwrap")
	for code, wantErr := range map[int]bool{1: false, 2: true} {
		t.Run(fmt.Sprint("exit ", code), func(t *testing.T) {
			fake := newFake()
			fake.Errors[pkill.String()] = shelltest.Exit(pkill, code)
			err := newXwinwrap(fake, shelltest.NewStore(), x11.Unmanaged, nil).Clear()
			if (err != nil) != wantErr {
				t.Errorf("Clear() = %v, wantErr %v", err, wantErr)
			}
		})
	}
}
