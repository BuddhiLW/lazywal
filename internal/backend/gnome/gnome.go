// Package gnome plays video wallpapers on GNOME Shell (Wayland and X11)
// through the Hanabi extension, which renders the video inside the shell.
package gnome

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

// Hanabi identifiers.
const (
	UUID   = "hanabi-extension@jeffshee.github.io"
	Schema = "io.github.jeffshee.hanabi-extension"
	URL    = "https://github.com/jeffshee/gnome-ext-hanabi"
)

// stateKey is where the first Set keeps what Clear must put back, so a later
// lazywal process can undo it.
const stateKey = "lazywal_gnome_saved"

// savedKeys are the Hanabi settings Set changes, in the order Clear restores
// them.
var savedKeys = []string{"video-path", "change-wallpaper"}

// Hanabi is the GNOME backend. The shell renders the video, so lazywal only
// changes Hanabi's settings and toggles the extension: there is no process
// to track.
type Hanabi struct {
	// SchemaDirs are tried with gsettings --schemadir when Hanabi's schema is
	// not in the global schema path: an extensions.gnome.org install keeps it
	// inside the extension directory.
	SchemaDirs []string

	run   shell.Runner
	store shell.Store
	logf  func(string, ...any)
}

var _ wallpaper.Backend = (*Hanabi)(nil)

// New returns a Hanabi backend that runs commands through r, keeps the
// settings to restore in st and reports progress to logf, which may be nil.
func New(r shell.Runner, st shell.Store, logf func(string, ...any)) *Hanabi {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local/share/gnome-shell/extensions", UUID, "schemas"))
	}
	dirs = append(dirs, filepath.Join("/usr/share/gnome-shell/extensions", UUID, "schemas"))
	return &Hanabi{SchemaDirs: dirs, run: r, store: st, logf: logf}
}

func (*Hanabi) Name() string { return "gnome" }

func (*Hanabi) Describe() string {
	return "GNOME (Wayland/X11) via the Hanabi extension (" + URL + ")"
}

// Check reports the missing programs, extension and schema together. The
// extension can only be queried with gnome-extensions, and the schema only
// matters once the extension is installed.
func (h *Hanabi) Check() error {
	req := wallpaper.Require(h.run.LookPath)
	hasCLI := req.Programs("gnome-extensions")
	hasGsettings := req.Programs("gsettings")
	if hasCLI && req.That(h.run.Run(extensionInfo(UUID)) == nil,
		"Hanabi GNOME extension ("+URL+"; log out and back in after installing)") && hasGsettings {
		_, err := h.settings()
		req.That(err == nil, "Hanabi settings schema "+Schema)
	}
	return req.Err()
}

// settings returns the gsettings builder that reaches Hanabi's schema: the
// global schema path first (meson installs), then each of SchemaDirs.
func (h *Hanabi) settings() (gsettings, error) {
	candidates := []gsettings{{}}
	for _, dir := range h.SchemaDirs {
		candidates = append(candidates, gsettings{}.schemaDir(dir))
	}
	for _, g := range candidates {
		if h.run.Run(g.listKeys(Schema)) == nil {
			return g, nil
		}
	}
	return gsettings{}, fmt.Errorf("cannot find the Hanabi settings schema %s", Schema)
}

// saved is what the first Set found, for Clear to put back. Settings are kept
// as gsettings get prints them (GVariant text), which gsettings set accepts
// back unchanged.
type saved struct {
	Enabled  bool              `json:"enabled"`
	Settings map[string]string `json:"settings"`
}

// Set saves the user's Hanabi state the first time only: a later Set would
// record lazywal's own video as the one to restore.
func (h *Hanabi) Set(v wallpaper.Video) error {
	g, err := h.settings()
	if err != nil {
		return err
	}
	if h.store.Get(stateKey) == "" {
		s, err := h.capture(g)
		if err != nil {
			return err
		}
		b, _ := json.Marshal(s)
		h.store.Set(stateKey, string(b))
	}
	for _, c := range []shell.Cmd{
		g.set(Schema, "video-path", gvariantString(v.Path())),
		// Hanabi's own slideshow would replace the video we just chose.
		g.set(Schema, "change-wallpaper", "false"),
		extensionEnable(UUID),
	} {
		if err := h.run.Run(c); err != nil {
			return err
		}
	}
	h.logf("Hanabi now playing %s", v)
	return nil
}

func (h *Hanabi) capture(g gsettings) (saved, error) {
	info, err := h.run.Output(extensionInfo(UUID))
	if err != nil {
		return saved{}, err
	}
	s := saved{Enabled: enabled(info), Settings: map[string]string{}}
	for _, key := range savedKeys {
		value, err := h.run.Output(g.get(Schema, key))
		if err != nil {
			return saved{}, err
		}
		if value == "" {
			return saved{}, fmt.Errorf("gsettings get %s %s: empty value", Schema, key)
		}
		s.Settings[key] = value
	}
	return s, nil
}

// Clear puts back what the first Set changed: Hanabi's settings and whether
// the extension was enabled. With nothing saved (never set, or already
// cleared) it runs nothing, so it succeeds even without Hanabi installed. A
// failed restore keeps the saved state so the next Clear can finish it.
func (h *Hanabi) Clear() error {
	raw := h.store.Get(stateKey)
	if raw == "" {
		return nil
	}
	var s saved
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		// The originals are lost, but lazywal did set a video: stop it.
		h.logf("Cannot read the saved Hanabi settings (%v); disabling the extension", err)
		if err := h.run.Run(extensionDisable(UUID)); err != nil {
			return err
		}
		h.store.Set(stateKey, "")
		return nil
	}
	g, err := h.settings()
	if err != nil {
		return err
	}
	// Disable before restoring, so the previous video never flashes on screen.
	if !s.Enabled {
		h.logf("Disabling the Hanabi extension")
		if err := h.run.Run(extensionDisable(UUID)); err != nil {
			return err
		}
	}
	for _, key := range savedKeys {
		value, ok := s.Settings[key]
		if !ok {
			continue
		}
		if err := h.run.Run(g.set(Schema, key, value)); err != nil {
			return err
		}
	}
	h.store.Set(stateKey, "")
	h.logf("Restored the previous Hanabi settings")
	return nil
}

// enabled reads gnome-extensions info. GNOME 45+ prints "Enabled: Yes";
// older releases print only "State: ENABLED". Labels and Yes/No are
// translated but the state is not, so the state (always the last line) is
// the fallback.
func enabled(info string) bool {
	lines := strings.Split(strings.TrimSpace(info), "\n")
	for _, line := range lines {
		if label, value, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(label) == "Enabled" {
			return strings.TrimSpace(value) == "Yes"
		}
	}
	_, state, _ := strings.Cut(lines[len(lines)-1], ":")
	switch strings.TrimSpace(state) {
	case "ENABLED", "ACTIVE", "ACTIVATING":
		return true
	}
	return false
}

// gvariantString renders s as a GVariant string literal. gsettings set
// parses its value as GVariant text and only falls back to a plain string
// when that fails, so an unquoted value could be misread.
func gvariantString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// gsettings builds gsettings commands, optionally reading schemas from a
// directory outside the global schema path.
type gsettings struct{ dir string }

func (g gsettings) schemaDir(dir string) gsettings {
	g.dir = dir
	return g
}

func (g gsettings) listKeys(schema string) shell.Cmd { return g.build("list-keys", schema) }

// get prints a key's value as GVariant text.
func (g gsettings) get(schema, key string) shell.Cmd { return g.build("get", schema, key) }

// set takes value as GVariant text.
func (g gsettings) set(schema, key, value string) shell.Cmd {
	return g.build("set", schema, key, value)
}

func (g gsettings) build(args ...string) shell.Cmd {
	if g.dir != "" {
		args = append([]string{"--schemadir", g.dir}, args...)
	}
	return shell.Command("gsettings", args...)
}

// extensionInfo succeeds only when the extension is installed.
func extensionInfo(uuid string) shell.Cmd { return shell.Command("gnome-extensions", "info", uuid) }

func extensionEnable(uuid string) shell.Cmd {
	return shell.Command("gnome-extensions", "enable", uuid)
}

func extensionDisable(uuid string) shell.Cmd {
	return shell.Command("gnome-extensions", "disable", uuid)
}
