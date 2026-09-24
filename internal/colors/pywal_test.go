package colors_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/BuddhiLW/lazywal/internal/colors"
	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

// frameWriter is a Fake whose ffmpeg, like the real one, writes the frame it
// is asked for (when writes allows it), so Apply finds a frame to hand to wal.
type frameWriter struct {
	*shelltest.Fake
	writes func(c shell.Cmd) bool
}

func (w *frameWriter) Run(c shell.Cmd) error {
	if err := w.Fake.Run(c); err != nil || c.Name != "ffmpeg" {
		return err
	}
	if w.writes != nil && !w.writes(c) {
		return nil
	}
	return os.WriteFile(c.Args[len(c.Args)-1], []byte("png"), 0o600)
}

// setup returns a Pywal whose ffmpeg writes every frame, whose ffprobe
// reports a 12.5s video and whose frames go into a fresh directory.
func setup(t *testing.T) (*colors.Pywal, *frameWriter, string) {
	t.Helper()
	w := &frameWriter{Fake: shelltest.New()}
	w.Outputs["ffprobe"] = "12.500000"
	tmp := t.TempDir()
	p := colors.New(w)
	p.TempDir = tmp
	p.Pick = func(limit time.Duration) time.Duration {
		if limit != 12500*time.Millisecond {
			t.Errorf("Pick(%v), want Pick(12.5s)", limit)
		}
		return 3*time.Second + 250*time.Millisecond + 999*time.Microsecond
	}
	return p, w, tmp
}

func probe(v wallpaper.Video) shell.Cmd {
	return shell.Command("ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", v.Path())
}

func ffmpeg(v wallpaper.Video, at, frame string) shell.Cmd {
	return shell.Command("ffmpeg", "-y", "-loglevel", "error", "-ss", at, "-i", v.Path(),
		"-frames:v", "1", "-q:v", "2", frame)
}

func wal(frame string) shell.Cmd { return shell.Command("wal", "-n", "-i", frame) }

// framePath returns the output of the first ffmpeg call, checking it is a
// frame in a lazywal directory of tmp.
func framePath(t *testing.T, f *shelltest.Fake, tmp string) string {
	t.Helper()
	for _, c := range f.Calls {
		if c.Name != "ffmpeg" {
			continue
		}
		frame := c.Args[len(c.Args)-1]
		dir := filepath.Dir(frame)
		if filepath.Base(frame) != "frame.png" || filepath.Dir(dir) != tmp ||
			!strings.HasPrefix(filepath.Base(dir), "lazywal-") {
			t.Fatalf("frame = %q, want %s/lazywal-*/frame.png", frame, tmp)
		}
		return frame
	}
	t.Fatalf("ffmpeg never ran: %q", f.Commands())
	return ""
}

func assertCalls(t *testing.T, f *shelltest.Fake, want ...shell.Cmd) {
	t.Helper()
	if !reflect.DeepEqual(f.Calls, want) {
		got := f.Commands()
		exp := make([]string, len(want))
		for i, c := range want {
			exp[i] = c.String()
		}
		t.Errorf("calls:\n got %q\nwant %q", got, exp)
	}
}

func assertEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("%s still holds %d entries (first %q); the frame directory was not removed",
			dir, len(entries), entries[0].Name())
	}
}

func TestApply(t *testing.T) {
	p, w, tmp := setup(t)
	v := backendtest.Video(t)
	if err := p.Apply(v); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	frame := framePath(t, w.Fake, tmp)
	assertCalls(t, w.Fake, probe(v), ffmpeg(v, "3.250", frame), wal(frame))
	assertEmpty(t, tmp)
}

func TestApplyPassesPathAsOneArgument(t *testing.T) {
	p, w, _ := setup(t)
	v := backendtest.Video(t)
	if !strings.ContainsAny(v.Path(), ` "'`) {
		t.Fatalf("test video %q has no spaces or quotes", v.Path())
	}
	if err := p.Apply(v); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	for _, c := range w.Calls[:2] {
		n := 0
		for _, a := range c.Args {
			if a == v.Path() {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s: video path appears %d times as a whole argument, want 1", c.Name, n)
		}
	}
}

func TestApplyRemovesFrameWhenWalFails(t *testing.T) {
	p, w, tmp := setup(t)
	w.Errors["wal"] = shelltest.Exit(shell.Command("wal"), 1)
	err := p.Apply(backendtest.Video(t))
	var exitErr *shell.ExitError
	if !errors.As(err, &exitErr) || exitErr.Cmd.Name != "wal" {
		t.Fatalf("Apply() = %v, want wal's exit error", err)
	}
	assertEmpty(t, tmp)
}

func TestApplyFFmpegFailure(t *testing.T) {
	p, w, tmp := setup(t)
	w.Errors["ffmpeg"] = shelltest.Exit(shell.Command("ffmpeg"), 1)
	v := backendtest.Video(t)
	err := p.Apply(v)
	var exitErr *shell.ExitError
	if !errors.As(err, &exitErr) || exitErr.Cmd.Name != "ffmpeg" {
		t.Fatalf("Apply() = %v, want ffmpeg's exit error", err)
	}
	frame := framePath(t, w.Fake, tmp)
	assertCalls(t, w.Fake, probe(v), ffmpeg(v, "3.250", frame))
	assertEmpty(t, tmp)
}

func TestApplyProbeFailure(t *testing.T) {
	p, w, tmp := setup(t)
	w.Errors["ffprobe"] = shelltest.Exit(shell.Command("ffprobe"), 1)
	v := backendtest.Video(t)
	err := p.Apply(v)
	var exitErr *shell.ExitError
	if !errors.As(err, &exitErr) || exitErr.Cmd.Name != "ffprobe" {
		t.Fatalf("Apply() = %v, want ffprobe's exit error", err)
	}
	assertCalls(t, w.Fake, probe(v))
	assertEmpty(t, tmp)
}

func TestApplyUnparsableDuration(t *testing.T) {
	for _, out := range []string{"", "garbage", "1:02:03.5", "-1", "NaN", "Inf", "1e300"} {
		t.Run(strconv.Quote(out), func(t *testing.T) {
			p, w, tmp := setup(t)
			w.Outputs["ffprobe"] = out
			v := backendtest.Video(t)
			err := p.Apply(v)
			if err == nil || !strings.Contains(err.Error(), "duration") {
				t.Fatalf("Apply() = %v, want a duration error", err)
			}
			assertCalls(t, w.Fake, probe(v))
			assertEmpty(t, tmp)
		})
	}
}

// Still images have no duration ("N/A") and a one-frame GIF has a zero one:
// the first frame is the only choice, so Pick is not consulted.
func TestApplyWithoutDurationTakesFirstFrame(t *testing.T) {
	for _, out := range []string{"N/A", "0.000000"} {
		t.Run(out, func(t *testing.T) {
			p, w, tmp := setup(t)
			w.Outputs["ffprobe"] = out
			p.Pick = func(limit time.Duration) time.Duration {
				t.Errorf("Pick(%v) called for a video without duration", limit)
				return 0
			}
			v := backendtest.Video(t)
			if err := p.Apply(v); err != nil {
				t.Fatalf("Apply() = %v", err)
			}
			frame := framePath(t, w.Fake, tmp)
			assertCalls(t, w.Fake, probe(v), ffmpeg(v, "0.000", frame), wal(frame))
		})
	}
}

// ffmpeg exits 0 without writing anything when seeking past the last frame.
func TestApplyRetriesFirstFrameWhenSeekPassesLastFrame(t *testing.T) {
	p, w, tmp := setup(t)
	w.writes = func(c shell.Cmd) bool { return c.Args[4] == "0.000" }
	v := backendtest.Video(t)
	if err := p.Apply(v); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	frame := framePath(t, w.Fake, tmp)
	assertCalls(t, w.Fake, probe(v), ffmpeg(v, "3.250", frame), ffmpeg(v, "0.000", frame), wal(frame))
	assertEmpty(t, tmp)
}

func TestApplyFailsWhenFFmpegWritesNoFrame(t *testing.T) {
	p, w, tmp := setup(t)
	w.writes = func(shell.Cmd) bool { return false }
	v := backendtest.Video(t)
	err := p.Apply(v)
	if err == nil || !strings.Contains(err.Error(), "no frame") {
		t.Fatalf("Apply() = %v, want a no-frame error", err)
	}
	frame := framePath(t, w.Fake, tmp)
	assertCalls(t, w.Fake, probe(v), ffmpeg(v, "3.250", frame), ffmpeg(v, "0.000", frame))
	assertEmpty(t, tmp)
}

func TestApplyDefaultPickStaysInRange(t *testing.T) {
	p, w, _ := setup(t)
	p.Pick = nil
	w.Outputs["ffprobe"] = "1.000000"
	v := backendtest.Video(t)
	for range 50 {
		w.Reset()
		if err := p.Apply(v); err != nil {
			t.Fatalf("Apply() = %v", err)
		}
		at, err := strconv.ParseFloat(w.Calls[1].Args[4], 64)
		if err != nil || at < 0 || at >= 1 {
			t.Fatalf("ffmpeg -ss %q, want seconds in [0, 1)", w.Calls[1].Args[4])
		}
	}
}

func TestApplyDefaultTempDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	p, w, _ := setup(t)
	p.TempDir = ""
	if err := p.Apply(backendtest.Video(t)); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	framePath(t, w.Fake, tmp)
	assertEmpty(t, tmp)
}

func TestCheck(t *testing.T) {
	if err := colors.New(shelltest.New()).Check(); err != nil {
		t.Errorf("Check() = %v, want nil", err)
	}
}

func TestCheckListsEveryMissingProgram(t *testing.T) {
	f := shelltest.New()
	for _, p := range []string{"ffmpeg", "ffprobe", "wal"} {
		f.Missing[p] = true
	}
	p := colors.New(f)
	backendtest.MissingDeps(t, p.Check(), "ffmpeg", "ffprobe", "wal")
	backendtest.MissingDeps(t, p.Apply(backendtest.Video(t)), "ffmpeg", "ffprobe", "wal")
	if len(f.Calls) != 0 {
		t.Errorf("ran %q without the programs to run", f.Commands())
	}
}

func TestCheckListsOnlyMissingPrograms(t *testing.T) {
	f := shelltest.New()
	f.Missing["wal"] = true
	backendtest.MissingDeps(t, colors.New(f).Check(), "wal")
}
