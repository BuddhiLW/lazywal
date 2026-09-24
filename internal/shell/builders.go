package shell

import (
	"os"
	"strconv"
	"strings"
)

// with returns a copy of s with more appended, so builders stay immutable:
// deriving two commands from one base never makes them share a backing array.
func with(s []string, more ...string) []string {
	out := make([]string, 0, len(s)+len(more))
	return append(append(out, s...), more...)
}

// MpvBuilder builds mpv invocations. Options are kept without their leading
// "--" so the same set can be handed to mpvpaper's -o.
type MpvBuilder struct {
	opts []string // e.g. "loop-file=inf"
	wid  string
	file string
}

// Mpv starts an mpv command.
func Mpv() MpvBuilder { return MpvBuilder{} }

// Option adds a raw mpv option, without the leading "--" (e.g. "hwdec=auto").
func (b MpvBuilder) Option(opt string) MpvBuilder {
	b.opts = with(b.opts, opt)
	return b
}

func (b MpvBuilder) Loop() MpvBuilder             { return b.Option("loop-file=inf") }
func (b MpvBuilder) NoAudio() MpvBuilder          { return b.Option("no-audio") }
func (b MpvBuilder) NoResumePlayback() MpvBuilder { return b.Option("no-resume-playback") }

// Panscan sets how much of the video may be cropped to fill the screen (0..1).
func (b MpvBuilder) Panscan(v float64) MpvBuilder {
	return b.Option("panscan=" + strconv.FormatFloat(v, 'f', -1, 64))
}

// Embed draws into an existing X11 window. wid may be XwinwrapWID, which
// xwinwrap replaces with its window ID.
func (b MpvBuilder) Embed(wid string) MpvBuilder {
	b.wid = wid
	return b
}

// File sets the media to play.
func (b MpvBuilder) File(path string) MpvBuilder {
	b.file = path
	return b
}

// Options returns the options without leading dashes, in insertion order.
func (b MpvBuilder) Options() []string { return with(b.opts) }

// Build returns the mpv command. The file comes after "--" so a path that
// starts with "-" is never parsed as an option.
func (b MpvBuilder) Build() Cmd {
	var args []string
	if b.wid != "" {
		// Two separate arguments, because xwinwrap only substitutes an
		// argument that is exactly "WID". mpv accepts a separate value only
		// in its legacy single-dash form: "--wid WID" makes it exit.
		args = with(args, "-wid", b.wid)
	}
	for _, o := range b.opts {
		args = with(args, "--"+o)
	}
	if b.file != "" {
		args = with(args, "--", b.file)
	}
	return Command("mpv", args...)
}

// XwinwrapWID is the argument xwinwrap replaces with the ID of its window.
const XwinwrapWID = "WID"

// XwinwrapBuilder builds xwinwrap invocations that wrap another command.
type XwinwrapBuilder struct{ flags []string }

// Xwinwrap starts an xwinwrap command.
func Xwinwrap() XwinwrapBuilder { return XwinwrapBuilder{} }

func (b XwinwrapBuilder) flag(f ...string) XwinwrapBuilder {
	b.flags = with(b.flags, f...)
	return b
}

// Geometry places the window: width x height at x,y.
func (b XwinwrapBuilder) Geometry(width, height, x, y int) XwinwrapBuilder {
	return b.flag("-g", strconv.Itoa(width)+"x"+strconv.Itoa(height)+"+"+strconv.Itoa(x)+"+"+strconv.Itoa(y))
}

func (b XwinwrapBuilder) IgnoreInput() XwinwrapBuilder { return b.flag("-ni") }
func (b XwinwrapBuilder) Below() XwinwrapBuilder       { return b.flag("-b") }
func (b XwinwrapBuilder) SkipTaskbar() XwinwrapBuilder { return b.flag("-st") }
func (b XwinwrapBuilder) SkipPager() XwinwrapBuilder   { return b.flag("-sp") }
func (b XwinwrapBuilder) Undecorated() XwinwrapBuilder { return b.flag("-un") }
func (b XwinwrapBuilder) Sticky() XwinwrapBuilder      { return b.flag("-s") }
func (b XwinwrapBuilder) NoFocus() XwinwrapBuilder     { return b.flag("-nf") }
func (b XwinwrapBuilder) Debug() XwinwrapBuilder       { return b.flag("-debug") }

// OverrideRedirect makes the window unmanaged by the window manager. Right
// for bare window managers (xmonad, i3...); on desktops with panels it
// covers the panels.
func (b XwinwrapBuilder) OverrideRedirect() XwinwrapBuilder { return b.flag("-ov") }

// DesktopType marks the window as _NET_WM_WINDOW_TYPE_DESKTOP so a desktop
// environment's window manager keeps it under its panels.
func (b XwinwrapBuilder) DesktopType() XwinwrapBuilder { return b.flag("-fdt") }

// Opacity sets the window opacity (0..1).
func (b XwinwrapBuilder) Opacity(v float64) XwinwrapBuilder {
	return b.flag("-o", strconv.FormatFloat(v, 'f', -1, 64))
}

// Wrap returns xwinwrap running inner inside its window.
func (b XwinwrapBuilder) Wrap(inner Cmd) Cmd {
	return Command("xwinwrap", with(with(b.flags, "--"), inner.Argv()...)...)
}

// MpvpaperBuilder builds mpvpaper invocations (wlr-layer-shell compositors).
type MpvpaperBuilder struct {
	opts   []string
	output string
	file   string
}

// MpvpaperAllOutputs plays on every output.
const MpvpaperAllOutputs = "ALL"

// Mpvpaper starts an mpvpaper command.
func Mpvpaper() MpvpaperBuilder { return MpvpaperBuilder{output: MpvpaperAllOutputs} }

// MpvOptions passes mpv options (as returned by MpvBuilder.Options) via -o.
func (b MpvpaperBuilder) MpvOptions(opts ...string) MpvpaperBuilder {
	b.opts = with(b.opts, opts...)
	return b
}

// Output selects the output (e.g. "DP-1"); defaults to all outputs.
func (b MpvpaperBuilder) Output(name string) MpvpaperBuilder {
	b.output = name
	return b
}

func (b MpvpaperBuilder) File(path string) MpvpaperBuilder {
	b.file = path
	return b
}

func (b MpvpaperBuilder) Build() Cmd {
	var args []string
	if len(b.opts) > 0 {
		// mpvpaper splits -o on spaces; our options never contain spaces.
		args = with(args, "-o", strings.Join(b.opts, " "))
	}
	return Command("mpvpaper", with(args, b.output, b.file)...)
}

// XrandrCurrent lists the current outputs without probing for new ones.
func XrandrCurrent() Cmd { return Command("xrandr", "--current") }

// uid scopes pgrep and pkill to this user: processes of another user
// logged in on the same machine are neither ours to count nor to kill.
var uid = strconv.Itoa(os.Getuid())

// Pgrep matches this user's processes named exactly name.
func Pgrep(name string) Cmd { return Command("pgrep", "-u", uid, "-x", name) }

// PgrepList lists this user's processes named exactly name, one per line:
// the PID, a space, then the command line with its arguments joined by
// spaces. It is for callers that must see how a process was started.
func PgrepList(name string) Cmd { return Command("pgrep", "-u", uid, "-a", "-x", name) }

// Pkill kills this user's processes named exactly name.
func Pkill(name string) Cmd { return Command("pkill", "-u", uid, "-x", name) }

// KillByName runs Pkill(name) and treats "no process matched" (exit 1) as success.
func KillByName(r Runner, name string) error {
	if err := r.Run(Pkill(name)); ExitCode(err) != 1 {
		return err
	}
	return nil
}

// IsRunning reports whether this user runs a process named exactly name.
func IsRunning(r Runner, name string) bool { return r.Run(Pgrep(name)) == nil }
