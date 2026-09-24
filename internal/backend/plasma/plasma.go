// Package plasma plays video wallpapers on KDE Plasma 6 (Wayland and X11)
// through the Smart Video Wallpaper Reborn plugin, configured with
// plasmashell's scripting D-Bus API.
package plasma

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

const (
	// Plugin is the Plasma wallpaper plugin id of Smart Video Wallpaper Reborn.
	Plugin = "luisbocanegra.smart.video.wallpaper.reborn"
	URL    = "https://github.com/luisbocanegra/plasma-smart-video-wallpaper-reborn"

	// imagePlugin is Plasma's stock wallpaper plugin, the fallback of Clear
	// for desktops it has no saved plugin for.
	imagePlugin = "org.kde.image"
)

const (
	// stateKey is where the first Set keeps each desktop's wallpaper, so a
	// later lazywal process can put it back.
	stateKey = "lazywal_plasma_saved"
	// restored replaces the snapshot once Clear has put it back. Clear then
	// does nothing, rather than taking the fallback path that would switch a
	// restored video wallpaper of the user's own to the image plugin.
	restored = "restored"

	// snapshotMark precedes the snapshot in the script's output, so the
	// reply parser can skip anything else plasmashell or dbus-send prints.
	snapshotMark = "lazywal-desktops:"
)

// configKeys are the plugin settings setScript writes, saved for desktops
// that already showed the plugin.
var configKeys = []string{"VideoUrls", "LastVideo", "LastVideoPosition"}

// Plasma is the KDE Plasma backend. plasmashell renders the video, so lazywal
// only switches the desktops' wallpaper plugin: there is no process to track.
type Plasma struct {
	// PluginDirs are where the plugin may be installed (per user, then system).
	PluginDirs []string
	// Exists reports whether path exists; Check probes PluginDirs with it.
	Exists func(path string) bool

	run   shell.Runner
	store shell.Store
	logf  func(string, ...any)
}

var _ wallpaper.Backend = (*Plasma)(nil)

// New returns a Plasma backend that runs commands through r, keeps the
// desktops' previous wallpapers in st and reports progress to logf, which
// may be nil.
func New(r shell.Runner, st shell.Store, logf func(string, ...any)) *Plasma {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local/share/plasma/wallpapers", Plugin))
	}
	dirs = append(dirs, filepath.Join("/usr/share/plasma/wallpapers", Plugin))
	return &Plasma{
		PluginDirs: dirs,
		Exists: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		},
		run:   r,
		store: st,
		logf:  logf,
	}
}

func (*Plasma) Name() string { return "plasma" }

func (*Plasma) Describe() string {
	return "KDE Plasma 6 (Wayland/X11) via Smart Video Wallpaper Reborn (" + URL + ")"
}

func (p *Plasma) Check() error {
	req := wallpaper.Require(p.run.LookPath)
	req.Programs("dbus-send")
	req.That(p.installed(), "Smart Video Wallpaper Reborn Plasma plugin ("+URL+")")
	return req.Err()
}

func (p *Plasma) installed() bool {
	for _, dir := range p.PluginDirs {
		if p.Exists(dir) {
			return true
		}
	}
	return false
}

// desktop is one Plasma desktop as the snapshot script reports it.
type desktop struct {
	Index  int    `json:"index"`
	Plugin string `json:"wallpaperPlugin"`
	// Config holds the plugin's configKeys, only for a desktop that already
	// showed the plugin. Values stay raw so Clear writes back the same types.
	Config map[string]json.RawMessage `json:"config,omitempty"`
}

// Set saves every desktop's wallpaper the first time only: a later Set would
// record lazywal's own video as the one to restore.
func (p *Plasma) Set(v wallpaper.Video) error {
	if s := p.store.Get(stateKey); s == "" || s == restored {
		if err := p.snapshot(); err != nil {
			return fmt.Errorf("reading the Plasma desktops before changing them: %w", err)
		}
	}
	if err := p.run.Run(evaluateScript(setScript(v.URI()))); err != nil {
		return err
	}
	p.logf("Plasma now playing %s", v)
	return nil
}

func (p *Plasma) snapshot() error {
	reply, err := p.run.Output(evaluateScript(snapshotScript()))
	if err != nil {
		return err
	}
	ds, err := parseSnapshot(reply)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(ds)
	p.store.Set(stateKey, string(b))
	return nil
}

// Clear puts every desktop showing the plugin back to what the first Set
// found. Without a snapshot (never set, or set by lazywal <= v1.4) it falls
// back to switching those desktops to the image plugin. A failed restore
// keeps the snapshot so the next Clear can finish it.
func (p *Plasma) Clear() error {
	var ds []desktop
	switch s := p.store.Get(stateKey); s {
	case restored:
		return nil
	case "":
		p.logf("Switching Plasma desktops showing the video plugin back to the image plugin")
		return p.run.Run(evaluateScript(clearScript()))
	default:
		if err := json.Unmarshal([]byte(s), &ds); err != nil {
			p.logf("Cannot read the saved Plasma desktops (%v); switching back to the image plugin", err)
			ds = nil
		}
	}
	p.logf("Restoring the previous wallpaper of Plasma desktops showing the video plugin")
	if err := p.run.Run(evaluateScript(restoreScript(ds))); err != nil {
		return err
	}
	p.store.Set(stateKey, restored)
	return nil
}

// parseSnapshot finds the snapshot in dbus-send --print-reply output, which
// wraps the returned string as `string "..."` after a header line. dbus-send
// prints the string verbatim; an escaped copy is decoded as a fallback.
func parseSnapshot(reply string) ([]desktop, error) {
	unescaped := strings.NewReplacer(`\\`, `\`, `\"`, `"`).Replace(reply)
	for _, text := range []string{reply, unescaped} {
		_, after, ok := strings.Cut(text, snapshotMark)
		if !ok {
			continue
		}
		var ds []desktop
		// Decode stops after the list, ignoring dbus-send's closing quote.
		if err := json.NewDecoder(strings.NewReader(after)).Decode(&ds); err == nil {
			return ds, nil
		}
	}
	return nil, fmt.Errorf("no desktop list in plasmashell's reply %q", reply)
}

// videoURL is one entry of the plugin's VideoUrls setting.
type videoURL struct {
	Filename string `json:"filename"`
	Enabled  bool   `json:"enabled"`
	Loop     bool   `json:"loop"`
}

// snapshotScript prints snapshotMark and a JSON list of every desktop's
// wallpaper plugin, with the plugin's configKeys where it is ours.
func snapshotScript() string {
	keys, _ := json.Marshal(configKeys)
	return fmt.Sprintf(`var keys = %[3]s;
var ds = desktops();
var out = [];
for (var i = 0; i < ds.length; i++) {
  var d = ds[i];
  var s = {index: i, wallpaperPlugin: d.wallpaperPlugin};
  if (d.wallpaperPlugin === %[2]s) {
    d.currentConfigGroup = ["Wallpaper", %[2]s, "General"];
    s.config = {};
    for (var k = 0; k < keys.length; k++) {
      s.config[keys[k]] = d.readConfig(keys[k]);
    }
  }
  out.push(s);
}
print(%[1]s + JSON.stringify(out));`, jsString(snapshotMark), jsString(Plugin), keys)
}

// setScript switches every desktop to the video plugin playing uri.
func setScript(uri string) string {
	videos, _ := json.Marshal([]videoURL{{Filename: uri, Enabled: true, Loop: true}})
	return fmt.Sprintf(`var ds = desktops();
for (var i = 0; i < ds.length; i++) {
  var d = ds[i];
  d.wallpaperPlugin = %[1]s;
  d.currentConfigGroup = ["Wallpaper", %[1]s, "General"];
  d.writeConfig("VideoUrls", %[2]s);
  d.writeConfig("LastVideo", %[3]s);
  d.writeConfig("LastVideoPosition", 0);
}`, jsString(Plugin), jsString(string(videos)), jsString(uri))
}

// restoreScript puts each desktop showing the plugin back to its plugin in
// saved (by index) and, where that was ours, writes back its saved settings.
// Desktops showing another plugin were changed after Set and are left
// alone; desktops missing from saved get clearScript's fallback.
func restoreScript(saved []desktop) string {
	byIndex := map[string]desktop{}
	for _, d := range saved {
		if d.Index >= 0 && d.Plugin != "" {
			byIndex[strconv.Itoa(d.Index)] = d
		}
	}
	// The values came from plasmashell, so they are embedded as JSON (a JS
	// expression), never spliced in as text.
	js, _ := json.Marshal(byIndex)
	return fmt.Sprintf(`var saved = %[1]s;
var ds = desktops();
for (var i = 0; i < ds.length; i++) {
  var d = ds[i];
  if (d.wallpaperPlugin !== %[2]s) {
    continue;
  }
  var s = saved[i];
  if (s === undefined) {
    d.wallpaperPlugin = %[3]s;
    continue;
  }
  d.wallpaperPlugin = s.wallpaperPlugin;
  if (s.config !== undefined) {
    d.currentConfigGroup = ["Wallpaper", %[2]s, "General"];
    for (var k in s.config) {
      if (s.config[k] !== null) {
        d.writeConfig(k, s.config[k]);
      }
    }
  }
}`, js, jsString(Plugin), jsString(imagePlugin))
}

// clearScript switches back to the image plugin only the desktops showing
// ours, so it is idempotent and never replaces a wallpaper lazywal did not set.
func clearScript() string {
	return fmt.Sprintf(`var ds = desktops();
for (var i = 0; i < ds.length; i++) {
  var d = ds[i];
  if (d.wallpaperPlugin === %s) {
    d.wallpaperPlugin = %s;
  }
}`, jsString(Plugin), jsString(imagePlugin))
}

// jsString renders s as a JavaScript string literal. A JSON string literal is
// one, so no value (e.g. a path with quotes) can break out of the script.
func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// evaluateScript runs script in plasmashell through its scripting D-Bus API.
// With --print-reply, dbus-send prints what the script print()s.
func evaluateScript(script string) shell.Cmd {
	return shell.Command("dbus-send", "--session", "--print-reply", "--dest=org.kde.plasmashell",
		"/PlasmaShell", "org.kde.PlasmaShell.evaluateScript", "string:"+script)
}
