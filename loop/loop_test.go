package loop

import (
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/BuddhiLW/bonzai"
	"github.com/BuddhiLW/lazywal/internal/backend/x11"
	"github.com/BuddhiLW/lazywal/internal/colors"
	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

func TestParseSetArgs(t *testing.T) {
	tests := []struct {
		args    []string
		want    setArgs
		wantErr bool
	}{
		{[]string{"v.mp4"}, setArgs{path: "v.mp4"}, false},
		{[]string{"v.mp4", "colors"}, setArgs{path: "v.mp4", colors: true}, false},
		{[]string{"v.mp4", "pywal"}, setArgs{path: "v.mp4", colors: true}, false},
		{[]string{"v.mp4", "update-pywal"}, setArgs{path: "v.mp4", colors: true}, false},
		// Older scripts pass a display size; it is accepted and ignored.
		{[]string{"v.mp4", "display", "2560x1080", "colors"}, setArgs{path: "v.mp4", display: "2560x1080", colors: true}, false},
		{[]string{"v.mp4", "display", "2560x1080"}, setArgs{path: "v.mp4", display: "2560x1080"}, false},
		{[]string{"v.mp4", "foo"}, setArgs{}, true},
		{[]string{"v.mp4", "display"}, setArgs{}, true},
		{nil, setArgs{}, true},
	}
	for _, tt := range tests {
		got, err := parseSetArgs(tt.args)
		if (err != nil) != tt.wantErr || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseSetArgs(%q) = %+v, %v; want %+v, err=%v", tt.args, got, err, tt.want, tt.wantErr)
		}
	}
}

// TestCmdTreeValid catches bonzai developer errors (e.g. Short too long)
// that otherwise only show up when the command runs.
func TestCmdTreeValid(t *testing.T) {
	var walk func(c *bonzai.Cmd)
	walk = func(c *bonzai.Cmd) {
		if err := c.Validate(); err != nil {
			t.Errorf("%s: %v", c.Name, err)
		}
		for _, sub := range c.Cmds {
			walk(sub)
		}
	}
	walk(Cmd)
}

type recordingBackend struct {
	sets   []wallpaper.Video
	clears int
}

func (*recordingBackend) Name() string                  { return "fake" }
func (*recordingBackend) Describe() string              { return "fake backend" }
func (*recordingBackend) Check() error                  { return nil }
func (b *recordingBackend) Set(v wallpaper.Video) error { b.sets = append(b.sets, v); return nil }
func (b *recordingBackend) Clear() error                { b.clears++; return nil }

// withFakeApp points the commands at an in-memory app whose only backend
// records calls.
func withFakeApp(t *testing.T) (*recordingBackend, *shelltest.Fake) {
	t.Helper()
	b := &recordingBackend{}
	reg := wallpaper.NewRegistry()
	reg.Register(b, wallpaper.Rule{})
	runner := shelltest.New()
	a := &app{
		Service: &wallpaper.Service{Registry: reg, State: shelltest.NewStore()},
		Colors:  &colors.Pywal{Runner: runner, TempDir: t.TempDir()},
	}
	old := newApp
	newApp = func() (*app, error) { return a, nil }
	t.Cleanup(func() { newApp = old })
	return b, runner
}

func TestSetAndClearCommands(t *testing.T) {
	b, runner := withFakeApp(t)
	v := backendtest.Video(t)

	if err := LoopCmd.Do(LoopCmd, v.Path(), "display", "1920x1080"); err != nil {
		t.Fatal(err)
	}
	if len(b.sets) != 1 || b.sets[0] != v {
		t.Fatalf("sets = %v, want [%v]", b.sets, v)
	}
	if len(runner.Calls) != 0 {
		t.Errorf("set without colors ran %q", runner.Commands())
	}

	if err := ClearCmd.Do(ClearCmd); err != nil {
		t.Fatal(err)
	}
	if b.clears != 1 {
		t.Errorf("clears = %d, want 1", b.clears)
	}
}

func TestSetColorsRunsPywal(t *testing.T) {
	_, runner := withFakeApp(t)
	runner.Outputs["ffprobe"] = "10.0"
	runner.Effects["ffmpeg"] = func(c shell.Cmd) error { // the real one writes the frame
		return os.WriteFile(c.Args[len(c.Args)-1], []byte("png"), 0o600)
	}
	v := backendtest.Video(t)

	if err := LoopCmd.Do(LoopCmd, v.Path(), "colors"); err != nil {
		t.Fatal(err)
	}
	ranWal := false
	for _, c := range runner.Calls {
		ranWal = ranWal || c.Name == "wal"
	}
	if !ranWal {
		t.Errorf("colors did not run wal: %q", runner.Commands())
	}
}

func TestSetRejectsMissingFile(t *testing.T) {
	b, _ := withFakeApp(t)
	if err := LoopCmd.Do(LoopCmd, "/does/not/exist.mp4"); err == nil {
		t.Fatal("expected an error for a missing file")
	}
	if len(b.sets) != 0 {
		t.Error("backend was called for a missing file")
	}
}

func TestPywalWithoutWallpaper(t *testing.T) {
	withFakeApp(t)
	if err := PywalCmd.Do(PywalCmd); err == nil {
		t.Fatal("expected an error when no wallpaper was set")
	}
}

func TestAppErrorsSurface(t *testing.T) {
	old := newApp
	newApp = func() (*app, error) { return nil, errors.New("bad backends.json") }
	t.Cleanup(func() { newApp = old })
	v := backendtest.Video(t)
	for _, c := range []*bonzai.Cmd{LoopCmd, ClearCmd, PywalCmd, BackendsCmd, DepsCmd} {
		args := []string{}
		if c == LoopCmd {
			args = []string{v.Path()}
		}
		if err := c.Do(c, args...); err == nil || err.Error() != "bad backends.json" {
			t.Errorf("%s: err = %v, want the app error", c.Name, err)
		}
	}
}

func TestShowHelp(t *testing.T) {
	if err := showHelp(Cmd); err != nil {
		t.Errorf("showHelp returned error: %v", err)
	}
	if err := HelpCmd.Do(HelpCmd); err != nil {
		t.Errorf("HelpCmd.Do returned error: %v", err)
	}
}

type failingClear struct{ recordingBackend }

func (*failingClear) Clear() error { return errors.New("xfdesktop would not start") }

func TestClearReportsFailureWithoutClaimingSuccess(t *testing.T) {
	b := &failingClear{}
	reg := wallpaper.NewRegistry()
	reg.Register(b, wallpaper.Rule{})
	a := &app{Service: &wallpaper.Service{Registry: reg, State: shelltest.NewStore()}}
	old := newApp
	newApp = func() (*app, error) { return a, nil }
	t.Cleanup(func() { newApp = old })

	out := captureStdout(t, func() {
		if err := ClearCmd.Do(ClearCmd); err == nil {
			t.Error("Clear succeeded although the backend failed")
		}
	})
	if strings.Contains(out, "Cleared") {
		t.Errorf("printed %q although Clear failed", out)
	}
}

func TestAdoptLegacyState(t *testing.T) {
	store := shelltest.NewStore()
	store.Set(x11.PIDsKey, "4242")
	adoptLegacyState(store)
	if got := store.Get(wallpaper.StateBackend); got != "xwinwrap" {
		t.Errorf("backend = %q, want xwinwrap for v1.4.3 state", got)
	}

	store.Set(wallpaper.StateBackend, "gnome")
	adoptLegacyState(store)
	if got := store.Get(wallpaper.StateBackend); got != "gnome" {
		t.Errorf("backend = %q, a recorded backend must win", got)
	}
}

func TestKeepRetiredReachable(t *testing.T) {
	store := shelltest.NewStore()
	store.Set(wallpaper.StateBackend, "gone")
	store.Set("lazywal_custom_gone_pids", "7/fake-7")
	reg := wallpaper.NewRegistry()
	keepRetiredReachable(reg, shelltest.New(), store)
	if !reg.Has("gone") {
		t.Fatal("removed custom backend with live players is not reachable")
	}

	empty := wallpaper.NewRegistry()
	store.Set("lazywal_custom_gone_pids", "")
	keepRetiredReachable(empty, shelltest.New(), store)
	if empty.Has("gone") {
		t.Error("registered a stand-in although nothing is tracked")
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}
