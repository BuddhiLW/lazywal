// Package colors derives a color scheme from the wallpaper video with pywal,
// so terminals and bars match what is playing on the desktop.
package colors

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

// Pywal feeds a random frame of a video to pywal. The frame is only needed
// while wal reads it, so it lives in a private temporary directory that is
// always removed.
type Pywal struct {
	Runner shell.Runner
	// TempDir is where the frame's directory is created; "" means os.TempDir().
	TempDir string
	// Pick chooses the frame's timestamp in [0, limit). nil picks uniformly at
	// random; tests set it to get a deterministic frame.
	Pick func(limit time.Duration) time.Duration
}

// New returns a Pywal that runs ffprobe, ffmpeg and wal through r.
func New(r shell.Runner) *Pywal { return &Pywal{Runner: r} }

// Check reports every missing program at once, so one install fixes them all.
func (p *Pywal) Check() error {
	req := wallpaper.Require(p.Runner.LookPath)
	req.Programs("ffmpeg", "ffprobe", "wal")
	return req.Err()
}

// errNoFrame reports that ffmpeg succeeded without writing a frame, which it
// does when asked to seek past the last one.
var errNoFrame = errors.New("ffmpeg wrote no frame")

// Apply sets pywal's color scheme from a random frame of v, leaving the
// wallpaper itself to lazywal.
func (p *Pywal) Apply(v wallpaper.Video) (err error) {
	if err := p.Check(); err != nil {
		return err
	}

	out, err := p.Runner.Output(probeDuration(v.Path()))
	if err != nil {
		return fmt.Errorf("reading the duration of %s: %w", v, err)
	}
	length, err := parseDuration(out)
	if err != nil {
		return fmt.Errorf("reading the duration of %s: %w", v, err)
	}
	var at time.Duration
	if length > 0 {
		at = p.pick(length)
	}

	dir, err := os.MkdirTemp(p.TempDir, "lazywal-")
	if err != nil {
		return fmt.Errorf("creating a directory for the frame: %w", err)
	}
	defer func() {
		if rmErr := os.RemoveAll(dir); rmErr != nil {
			err = errors.Join(err, fmt.Errorf("removing %s: %w", dir, rmErr))
		}
	}()
	frame := filepath.Join(dir, "frame.png")

	err = p.extract(v.Path(), at, frame)
	if errors.Is(err, errNoFrame) && at > 0 {
		// The pick fell within a frame's length of the end.
		err = p.extract(v.Path(), 0, frame)
	}
	if err != nil {
		return err
	}

	if err := p.Runner.Run(walScheme(frame)); err != nil {
		return fmt.Errorf("generating colors with pywal: %w", err)
	}
	return nil
}

func (p *Pywal) pick(limit time.Duration) time.Duration {
	if p.Pick != nil {
		return p.Pick(limit)
	}
	return rand.N(limit)
}

// extract writes the frame of video at at to out, failing with errNoFrame
// when ffmpeg exits cleanly but writes nothing.
func (p *Pywal) extract(video string, at time.Duration, out string) error {
	if err := p.Runner.Run(extractFrame(video, at, out)); err != nil {
		return fmt.Errorf("extracting a frame at %ss: %w", seconds(at), err)
	}
	if _, err := os.Stat(out); err != nil {
		return fmt.Errorf("%w at %ss", errNoFrame, seconds(at))
	}
	return nil
}

// maxSeconds is the longest duration a time.Duration holds.
var maxSeconds = float64(math.MaxInt64 / int64(time.Second))

// parseDuration reads ffprobe's duration in seconds. ffprobe prints "N/A"
// for media without a duration (still images); that and a zero duration
// (a one-frame GIF) both mean the first frame is the only one to take.
func parseDuration(out string) (time.Duration, error) {
	out = strings.TrimSpace(out)
	if out == "N/A" {
		return 0, nil
	}
	secs, err := strconv.ParseFloat(out, 64)
	if err != nil || !(secs >= 0 && secs <= maxSeconds) { // also rejects NaN
		return 0, fmt.Errorf("ffprobe printed %q, want a duration in seconds", out)
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// seconds formats d for ffmpeg with millisecond precision, truncating so a
// time just before the end never rounds up past it.
func seconds(d time.Duration) string {
	return strconv.FormatFloat(float64(d.Milliseconds())/1000, 'f', 3, 64)
}

// probeDuration prints the container duration in seconds and nothing else,
// so no banner has to be parsed.
func probeDuration(path string) shell.Cmd {
	return shell.Command("ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path)
}

// extractFrame writes one high-quality frame of video at at to out. -ss goes
// before -i so ffmpeg seeks the input instead of decoding up to at.
func extractFrame(video string, at time.Duration, out string) shell.Cmd {
	return shell.Command("ffmpeg", "-y", "-loglevel", "error", "-ss", seconds(at), "-i", video,
		"-frames:v", "1", "-q:v", "2", out)
}

// walScheme generates a scheme from image. -n stops wal from setting image
// as the wallpaper: lazywal owns the wallpaper.
func walScheme(image string) shell.Cmd { return shell.Command("wal", "-n", "-i", image) }
