package x11

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

// DesktopLayerKeyPrefix plus a backend name is where Desktop remembers the
// state to restore.
const DesktopLayerKeyPrefix = "lazywal_desktop_layer_"

// Layer is a desktop environment's icon/desktop window. It sits above the
// root window and so covers the video unless turned off.
type Layer interface {
	// Hide turns the layer off and returns the state Restore needs to put it
	// back as it was before this Hide.
	Hide() (state string, err error)
	// Restore puts the layer back as state, returned by an earlier Hide,
	// describes.
	Restore(state string) error
	// Current reports whether state, possibly saved by another lazywal
	// process, still applies: whether restoring it now would put back what
	// the user had. A state bound to a login session is not current in
	// another one.
	Current(state string) bool
}

// Desktop decorates any Backend with hiding a desktop Layer while a video
// plays and restoring it on Clear.
type Desktop struct {
	inner             wallpaper.Backend
	name, description string
	layer             Layer
	store             shell.Store
	logf              func(string, ...any)
}

var _ wallpaper.Backend = (*Desktop)(nil)

// NewDesktop wraps inner. The restore state is kept in store so a later
// lazywal process can undo what this one hid; logf may be nil.
func NewDesktop(inner wallpaper.Backend, name, description string, layer Layer, store shell.Store, logf func(string, ...any)) *Desktop {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Desktop{inner: inner, name: name, description: description, layer: layer, store: store, logf: logf}
}

func (d *Desktop) Name() string     { return d.name }
func (d *Desktop) Describe() string { return d.description }
func (d *Desktop) Check() error     { return d.inner.Check() }

func (d *Desktop) stateKey() string { return DesktopLayerKeyPrefix + d.name }

// Set hides the layer every time: a new login, or the user, may have shown
// it again since the last Set. It keeps a saved state that is still current,
// though: once hidden, the layer would report its hidden state as the one
// to restore.
func (d *Desktop) Set(v wallpaper.Video) error {
	saved := d.store.Get(d.stateKey())
	state, err := d.layer.Hide()
	switch {
	case err != nil:
		d.logf("Could not hide the %s desktop layer, the video may be covered: %v", d.name, err)
	case saved == "" || !d.layer.Current(saved):
		d.store.Set(d.stateKey(), state)
	}
	return d.inner.Set(v)
}

// Clear stops the video and restores the layer if the saved state is still
// current; one from another session is dropped unrestored. The state is
// dropped even when restoring fails, so Clear stays safe to repeat.
func (d *Desktop) Clear() error {
	err := d.inner.Clear()
	if state := d.store.Get(d.stateKey()); state != "" {
		if d.layer.Current(state) {
			if rerr := d.layer.Restore(state); rerr != nil && err == nil {
				err = fmt.Errorf("restoring %s desktop layer: %w", d.name, rerr)
			}
		}
		d.store.Set(d.stateKey(), "")
	}
	return err
}

// GSettingsLayer hides a layer by setting a GSettings key to Off and restores
// the value it had before.
type GSettingsLayer struct {
	Runner           shell.Runner
	Schema, Key, Off string
}

func (l GSettingsLayer) Hide() (string, error) {
	prev, err := l.Runner.Output(gsettingsGet(l.Schema, l.Key))
	if err != nil {
		return "", err
	}
	if prev == "" {
		return "", fmt.Errorf("gsettings get %s %s: empty value", l.Schema, l.Key)
	}
	if err := l.Runner.Run(gsettingsSet(l.Schema, l.Key, l.Off)); err != nil {
		return "", err
	}
	return prev, nil
}

func (l GSettingsLayer) Restore(state string) error {
	return l.Runner.Run(gsettingsSet(l.Schema, l.Key, state))
}

// Current is always true: the value lives in the user's dconf database, so
// the one saved in an earlier session is still the one to restore.
func (GSettingsLayer) Current(string) bool { return true }

// gsettingsGet prints a key's value in GVariant text form, which
// gsettingsSet accepts back unchanged.
func gsettingsGet(schema, key string) shell.Cmd {
	return shell.Command("gsettings", "get", schema, key)
}

func gsettingsSet(schema, key, value string) shell.Cmd {
	return shell.Command("gsettings", "set", schema, key, value)
}

// ProcessLayer hides a layer drawn by a process (xfdesktop, pcmanfm...) by
// running Off, and restores it by starting the command line the process
// had, so session flags such as pcmanfm's --profile are kept.
//
// Its state is bound to the login session: the process it would restart
// belongs to the session it was captured in.
type ProcessLayer struct {
	Runner shell.Runner
	// Process is the program's kernel name, as pgrep -x matches it.
	Process string
	// DesktopFlag is the argument that makes Process draw the desktop.
	// When set, a Process running without it (e.g. pcmanfm as a plain file
	// manager) shows no desktop. Empty means running is showing.
	DesktopFlag string
	Off         shell.Cmd
	// Session identifies the current login session; nil means the boot,
	// logind's session ID and the X display.
	Session func() string
}

// layerNotShown is the state of a ProcessLayer that was not shown: there is
// nothing to restore, and nothing is lost by replacing it.
const layerNotShown = "not-shown"

// processState is the JSON state of a shown ProcessLayer.
type processState struct {
	Session string   `json:"session"`
	Argv    []string `json:"argv"`
}

// currentSession identifies this login session by the boot, logind's
// session ID and the X display. A part that cannot be read is left empty.
func currentSession() string {
	boot, _ := os.ReadFile("/proc/sys/kernel/random/boot_id")
	return strings.TrimSpace(string(boot)) + "/" + os.Getenv("XDG_SESSION_ID") + "/" + os.Getenv("DISPLAY")
}

func (l ProcessLayer) session() string {
	if l.Session == nil {
		return currentSession()
	}
	return l.Session()
}

func (l ProcessLayer) Hide() (string, error) {
	argv, err := l.shownBy()
	if err != nil {
		return "", err
	}
	if argv == nil {
		return layerNotShown, nil
	}
	if err := l.Runner.Run(l.Off); err != nil {
		return "", err
	}
	state, err := json.Marshal(processState{Session: l.session(), Argv: argv})
	return string(state), err
}

// shownBy returns the command line of the running Process that draws the
// desktop, or nil when none does. pgrep prints the arguments joined by
// spaces, so an argument containing a space comes back split in two;
// desktop command lines have none in practice.
func (l ProcessLayer) shownBy() ([]string, error) {
	out, err := l.Runner.Output(shell.PgrepList(l.Process))
	if shell.ExitCode(err) == 1 { // none running
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		argv := fields[1:]
		if l.DesktopFlag == "" || slices.Contains(argv[1:], l.DesktopFlag) {
			return argv, nil
		}
	}
	return nil, nil
}

func (l ProcessLayer) parse(state string) (processState, bool) {
	var s processState
	if json.Unmarshal([]byte(state), &s) != nil || len(s.Argv) == 0 {
		return processState{}, false
	}
	return s, true
}

// Current reports whether state was captured in this session. The
// not-shown state never is, so any later Hide replaces it.
func (l ProcessLayer) Current(state string) bool {
	s, ok := l.parse(state)
	return ok && s.Session == l.session()
}

// Restore starts the captured command line detached and untracked, since
// the desktop must outlive lazywal and never be killed as a wallpaper
// process. It does nothing for a state from another session.
func (l ProcessLayer) Restore(state string) error {
	s, ok := l.parse(state)
	if !ok || s.Session != l.session() {
		return nil
	}
	_, err := l.Runner.Start(shell.Command(s.Argv[0], s.Argv[1:]...))
	return err
}

// MateLayer hides Caja's desktop icons.
func MateLayer(r shell.Runner) GSettingsLayer {
	return GSettingsLayer{Runner: r, Schema: "org.mate.background", Key: "show-desktop-icons", Off: "false"}
}

// CinnamonLayer hides Nemo's desktop; the layout lists which monitors
// ("primary:others") get a desktop window.
func CinnamonLayer(r shell.Runner) GSettingsLayer {
	return GSettingsLayer{Runner: r, Schema: "org.nemo.desktop", Key: "desktop-layout", Off: "'false::false'"}
}

// XfceLayer stops xfdesktop, which draws the desktop whenever it runs.
func XfceLayer(r shell.Runner) ProcessLayer {
	return ProcessLayer{
		Runner:  r,
		Process: "xfdesktop",
		Off:     shell.Command("xfdesktop", "--quit"),
	}
}

// LXQtLayer turns off pcmanfm-qt's desktop.
func LXQtLayer(r shell.Runner) ProcessLayer { return pcmanfmLayer(r, "pcmanfm-qt") }

// LXDELayer turns off pcmanfm's desktop.
func LXDELayer(r shell.Runner) ProcessLayer { return pcmanfmLayer(r, "pcmanfm") }

// pcmanfmLayer turns off the desktop of a single-instance pcmanfm: the
// process keeps running (and keeps its command line) after --desktop-off,
// and restarting that command line hands --desktop back to it.
func pcmanfmLayer(r shell.Runner, program string) ProcessLayer {
	return ProcessLayer{
		Runner:      r,
		Process:     program,
		DesktopFlag: "--desktop",
		Off:         shell.Command(program, "--desktop-off"),
	}
}
