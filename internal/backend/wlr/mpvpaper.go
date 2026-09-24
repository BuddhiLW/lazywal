// Package wlr plays wallpapers on Wayland compositors that implement
// wlr-layer-shell (Hyprland, Sway, river, niri, Wayfire, labwc, COSMIC...),
// where a client can draw on the background layer itself.
package wlr

import (
	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

// pidsKey stores the mpvpaper processes this backend started, so a later
// lazywal process can stop them.
const pidsKey = "lazywal_mpvpaper_pids"

// Mpvpaper plays the video with mpvpaper on every output.
type Mpvpaper struct {
	runner shell.Runner
	procs  shell.Tracker
}

// NewMpvpaper returns the mpvpaper backend; s remembers the players it starts.
func NewMpvpaper(r shell.Runner, s shell.Store) *Mpvpaper {
	return &Mpvpaper{runner: r, procs: shell.Tracker{Runner: r, Store: s, Key: pidsKey}}
}

func (*Mpvpaper) Name() string { return "mpvpaper" }

func (*Mpvpaper) Describe() string {
	return "Wayland layer-shell compositors (Hyprland, Sway, river, niri, Wayfire, labwc, COSMIC...) via mpvpaper"
}

func (m *Mpvpaper) Check() error {
	req := wallpaper.Require(m.runner.LookPath)
	req.Programs("mpvpaper")
	return req.Err()
}

// Set stops every mpvpaper first, including ones lazywal did not track: a
// stale instance would stack a second background layer under the new one.
func (m *Mpvpaper) Set(v wallpaper.Video) error {
	if err := m.Clear(); err != nil {
		return err
	}
	_, err := m.procs.Spawn(play(v))
	return err
}

func (m *Mpvpaper) Clear() error {
	m.procs.KillAll()
	return shell.KillByName(m.runner, "mpvpaper")
}

// play loops v silently on all outputs, cropping it to fill each screen.
func play(v wallpaper.Video) shell.Cmd {
	opts := shell.Mpv().NoAudio().Loop().Panscan(1.0).NoResumePlayback().Options()
	return shell.Mpvpaper().MpvOptions(opts...).File(v.Path()).Build()
}
