package x11

import (
	"errors"
	"fmt"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

// PIDsKey is where Xwinwrap records its xwinwrap PIDs. It is the key older
// lazywal versions used, so a new version still stops what an old one started.
const PIDsKey = "lazywal_pids"

// legacyNames are the process names a bare PID recorded by lazywal <=
// v1.4.3 may have: it ran "bash -c 'xwinwrap ...'", where bash usually
// execs xwinwrap but may stay as its parent.
var legacyNames = []string{"xwinwrap", "bash"}

// XwinwrapConfig configures an Xwinwrap backend.
type XwinwrapConfig struct {
	Name, Description string
	// Mode places the window; nil means Unmanaged.
	Mode   WindowMode
	Runner shell.Runner
	Store  shell.Store
	// Monitors lists where to play; nil means Xrandr{Runner}.
	Monitors MonitorSource
	// Logf, when set, receives progress messages.
	Logf func(format string, args ...any)
}

// Xwinwrap plays a video with mpv inside one xwinwrap window per monitor,
// drawn beneath all other windows.
type Xwinwrap struct {
	name, description string
	mode              WindowMode
	runner            shell.Runner
	monitors          MonitorSource
	procs             shell.Tracker
	logf              func(string, ...any)
}

var _ wallpaper.Backend = (*Xwinwrap)(nil)

// NewXwinwrap returns an Xwinwrap backend configured by cfg.
func NewXwinwrap(cfg XwinwrapConfig) *Xwinwrap {
	x := &Xwinwrap{
		name:        cfg.Name,
		description: cfg.Description,
		mode:        cfg.Mode,
		runner:      cfg.Runner,
		monitors:    cfg.Monitors,
		procs:       shell.Tracker{Runner: cfg.Runner, Store: cfg.Store, Key: PIDsKey, Legacy: legacyNames},
		logf:        cfg.Logf,
	}
	if x.mode == nil {
		x.mode = Unmanaged
	}
	if x.monitors == nil {
		x.monitors = Xrandr{Runner: cfg.Runner}
	}
	if x.logf == nil {
		x.logf = func(string, ...any) {}
	}
	return x
}

func (x *Xwinwrap) Name() string     { return x.name }
func (x *Xwinwrap) Describe() string { return x.description }

func (x *Xwinwrap) Check() error {
	req := wallpaper.Require(x.runner.LookPath)
	req.Programs("xwinwrap", "mpv", "xrandr")
	return req.Err()
}

// Set replaces whatever this backend (or an older lazywal) was playing. It
// succeeds when at least one monitor shows the video.
func (x *Xwinwrap) Set(v wallpaper.Video) error {
	monitors, err := x.monitors.Monitors()
	if err != nil {
		return fmt.Errorf("getting monitor info: %w", err)
	}
	x.procs.KillAll()

	var errs []error
	for _, m := range monitors {
		pid, err := x.procs.Spawn(x.command(m, v))
		if err != nil {
			x.logf("Failed to start on monitor %s: %v", m.Name, err)
			errs = append(errs, fmt.Errorf("monitor %s: %w", m.Name, err))
			continue
		}
		x.logf("Started wallpaper on monitor %s (PID: %d)", m.Name, pid)
	}
	if len(errs) == len(monitors) {
		return fmt.Errorf("could not start wallpaper on any monitor: %w", errors.Join(errs...))
	}
	return nil
}

func (x *Xwinwrap) command(m Monitor, v wallpaper.Video) shell.Cmd {
	player := shell.Mpv().
		Embed(shell.XwinwrapWID).
		Loop().
		NoAudio().
		NoResumePlayback().
		Panscan(1.0).
		File(v.Path()).
		Build()
	return x.mode.Apply(shell.Xwinwrap().Geometry(m.Width, m.Height, m.X, m.Y)).Wrap(player)
}

// Clear kills the tracked windows and any stray xwinwrap left by a crashed
// or pre-tracking lazywal.
func (x *Xwinwrap) Clear() error {
	x.procs.KillAll()
	return shell.KillByName(x.runner, "xwinwrap")
}
