package plasma

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

const pluginDep = "Smart Video Wallpaper Reborn Plasma plugin"

// reply is what dbus-send --print-reply prints for the snapshot script on a
// three-desktop setup: an image, the video plugin with the user's own
// playlist, and a slideshow. dbus-send prints the string verbatim, so the
// JSON-encoded VideoUrls keeps its backslashes.
const reply = `method return time=1727089142.512345 sender=:1.27 -> destination=:1.312 serial=4211 reply_serial=2
   string "lazywal-desktops:[{"index":0,"wallpaperPlugin":"org.kde.image"},{"index":1,"wallpaperPlugin":"luisbocanegra.smart.video.wallpaper.reborn","config":{"VideoUrls":"[{\"filename\":\"file:///home/u/Videos/mine.mp4\",\"enabled\":true,\"loop\":false}]","LastVideo":"file:///home/u/Videos/mine.mp4","LastVideoPosition":42.5}},{"index":2,"wallpaperPlugin":"org.kde.slideshow"}]
"`

// userPlaylist is the VideoUrls value in reply.
var userPlaylist = []videoURL{{Filename: "file:///home/u/Videos/mine.mp4", Enabled: true}}

var snapshotCmd = evaluateScript(snapshotScript())

// newFake scripts plasmashell to answer the snapshot script with reply.
func newFake() *shelltest.Fake {
	f := shelltest.New()
	f.Outputs[snapshotCmd.String()] = reply
	return f
}

func newPlasma(t *testing.T, f *shelltest.Fake, st shell.Store, installed bool) *Plasma {
	p := New(f, st, t.Logf)
	p.PluginDirs = []string{"/user/plugin", "/system/plugin"}
	p.Exists = func(string) bool { return installed }
	return p
}

// scriptOf returns the script c evaluates, checking that it reaches
// dbus-send as one argv element.
func scriptOf(t *testing.T, c shell.Cmd) string {
	t.Helper()
	argv := c.Argv()
	prefix := []string{"dbus-send", "--session", "--print-reply", "--dest=org.kde.plasmashell",
		"/PlasmaShell", "org.kde.PlasmaShell.evaluateScript"}
	if len(argv) != len(prefix)+1 || !reflect.DeepEqual(argv[:len(prefix)], prefix) ||
		!strings.HasPrefix(argv[len(prefix)], "string:") {
		t.Fatalf("argv = %q, want %q followed by one string:<script>", argv, prefix)
	}
	return strings.TrimPrefix(argv[len(prefix)], "string:")
}

// evaluated returns the script of the single evaluateScript call f recorded.
func evaluated(t *testing.T, f *shelltest.Fake) string {
	t.Helper()
	if len(f.Calls) != 1 {
		t.Fatalf("calls = %q, want one dbus-send", f.Commands())
	}
	return scriptOf(t, f.Calls[0])
}

// configArg decodes the string literal the script passes to writeConfig(key, ...).
func configArg(t *testing.T, script, key string) string {
	t.Helper()
	call := "d.writeConfig(" + jsString(key) + ", "
	i := strings.Index(script, call)
	if i < 0 {
		t.Fatalf("script does not write %s:\n%s", key, script)
	}
	var s string
	if err := json.NewDecoder(strings.NewReader(script[i+len(call):])).Decode(&s); err != nil {
		t.Fatalf("%s argument is not a string literal: %v\n%s", key, err, script)
	}
	return s
}

// assertPlays checks that script points the plugin at uri, and only uri.
func assertPlays(t *testing.T, script, uri string) {
	t.Helper()
	if !strings.Contains(script, "d.wallpaperPlugin = "+jsString(Plugin)+";") {
		t.Errorf("script does not select the plugin:\n%s", script)
	}
	// VideoUrls is a JS string holding the plugin's JSON list.
	var videos []videoURL
	if err := json.Unmarshal([]byte(configArg(t, script, "VideoUrls")), &videos); err != nil {
		t.Fatalf("VideoUrls is not a JSON list: %v", err)
	}
	if want := []videoURL{{Filename: uri, Enabled: true, Loop: true}}; !reflect.DeepEqual(videos, want) {
		t.Errorf("VideoUrls = %+v, want %+v", videos, want)
	}
	if got := configArg(t, script, "LastVideo"); got != uri {
		t.Errorf("LastVideo = %q, want %q", got, uri)
	}
}

// savedIn decodes the per-desktop map restoreScript embeds, checking that it
// is one JSON value the script cannot be broken out of.
func savedIn(t *testing.T, script string) map[string]desktop {
	t.Helper()
	const decl = "var saved = "
	if !strings.HasPrefix(script, decl) {
		t.Fatalf("script does not start with %q:\n%s", decl, script)
	}
	dec := json.NewDecoder(strings.NewReader(script[len(decl):]))
	var saved map[string]desktop
	if err := dec.Decode(&saved); err != nil {
		t.Fatalf("saved desktops are not one JSON value: %v\n%s", err, script)
	}
	if rest := script[len(decl)+int(dec.InputOffset()):]; !strings.HasPrefix(rest, ";\nvar ds = desktops();") {
		t.Fatalf("script continues with %.40q after the saved desktops", rest)
	}
	return saved
}

// assertDesktops compares desktops, with config values compared as JSON text.
func assertDesktops(t *testing.T, got, want []desktop) {
	t.Helper()
	g, _ := json.Marshal(got)
	w, _ := json.Marshal(want)
	if string(g) != string(w) {
		t.Errorf("desktops =\n%s\nwant\n%s", g, w)
	}
}

// replyDesktops is what parseSnapshot must read from reply.
func replyDesktops(t *testing.T) []desktop {
	t.Helper()
	playlist, _ := json.Marshal(userPlaylist)
	return []desktop{
		{Index: 0, Plugin: imagePlugin},
		{Index: 1, Plugin: Plugin, Config: map[string]json.RawMessage{
			"VideoUrls":         json.RawMessage(jsString(string(playlist))),
			"LastVideo":         json.RawMessage(jsString("file:///home/u/Videos/mine.mp4")),
			"LastVideoPosition": json.RawMessage("42.5"),
		}},
		{Index: 2, Plugin: "org.kde.slideshow"},
	}
}

func TestContract(t *testing.T) {
	backendtest.ContractWithRunner(t, func(t *testing.T, r *shelltest.Fake, s *shelltest.Store) wallpaper.Backend {
		for k, v := range newFake().Outputs {
			r.Outputs[k] = v
		}
		return newPlasma(t, r, s, true)
	})
}

func TestNewDefaults(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	p := New(shelltest.New(), shelltest.NewStore(), nil)
	want := []string{"/home/u/.local/share/plasma/wallpapers/" + Plugin, "/usr/share/plasma/wallpapers/" + Plugin}
	if !reflect.DeepEqual(p.PluginDirs, want) {
		t.Errorf("PluginDirs = %q, want %q", p.PluginDirs, want)
	}
	dir := t.TempDir()
	if !p.Exists(dir) || p.Exists(filepath.Join(dir, "missing")) {
		t.Error("Exists does not report whether a path exists")
	}
}

func TestCheckListsEveryMissingDependency(t *testing.T) {
	f := shelltest.New()
	f.Missing["dbus-send"] = true
	backendtest.MissingDeps(t, newPlasma(t, f, shelltest.NewStore(), false).Check(), "dbus-send", pluginDep)
}

func TestCheckFindsPluginInAnyDir(t *testing.T) {
	p := newPlasma(t, shelltest.New(), shelltest.NewStore(), false)
	p.Exists = func(path string) bool { return path == "/system/plugin" }
	if err := p.Check(); err != nil {
		t.Errorf("Check() = %v, want nil with the plugin installed system-wide", err)
	}
}

func TestSnapshotScript(t *testing.T) {
	script := snapshotScript()
	for _, want := range []string{
		`var keys = ["VideoUrls","LastVideo","LastVideoPosition"];`,
		"var s = {index: i, wallpaperPlugin: d.wallpaperPlugin};",
		"if (d.wallpaperPlugin === " + jsString(Plugin) + ") {",
		"s.config[keys[k]] = d.readConfig(keys[k]);",
		"print(" + jsString(snapshotMark) + " + JSON.stringify(out));",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script lacks %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, "writeConfig") || regexp.MustCompile(`wallpaperPlugin\s*=[^=]`).MatchString(script) {
		t.Errorf("snapshot script changes a desktop:\n%s", script)
	}
}

func TestParseSnapshot(t *testing.T) {
	want := replyDesktops(t)
	for _, tc := range []struct{ name, reply string }{
		{"dbus-send reply", reply},
		{"other lines printed first", strings.Replace(reply, `string "`, "string \"js: some warning\n", 1)},
		{"escaped string", `method return time=1 sender=:1.2 -> destination=:1.3 serial=4 reply_serial=5
   string "lazywal-desktops:[{\"index\":0,\"wallpaperPlugin\":\"org.kde.image\"},{\"index\":1,\"wallpaperPlugin\":\"luisbocanegra.smart.video.wallpaper.reborn\",\"config\":{\"VideoUrls\":\"[{\\\"filename\\\":\\\"file:///home/u/Videos/mine.mp4\\\",\\\"enabled\\\":true,\\\"loop\\\":false}]\",\"LastVideo\":\"file:///home/u/Videos/mine.mp4\",\"LastVideoPosition\":42.5}},{\"index\":2,\"wallpaperPlugin\":\"org.kde.slideshow\"}]"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSnapshot(tc.reply)
			if err != nil {
				t.Fatal(err)
			}
			assertDesktops(t, got, want)
			var videos string
			if err := json.Unmarshal(got[1].Config["VideoUrls"], &videos); err != nil {
				t.Fatalf("VideoUrls = %s, want a JSON string", got[1].Config["VideoUrls"])
			}
			var playlist []videoURL
			if err := json.Unmarshal([]byte(videos), &playlist); err != nil || !reflect.DeepEqual(playlist, userPlaylist) {
				t.Errorf("VideoUrls playlist = %+v (%v), want %+v", playlist, err, userPlaylist)
			}
		})
	}
}

func TestParseSnapshotRejectsOtherReplies(t *testing.T) {
	for _, r := range []string{
		"",
		`method return time=1 sender=:1.2 -> destination=:1.3 serial=4 reply_serial=5
   string ""`,
		`method return time=1 sender=:1.2 -> destination=:1.3 serial=4 reply_serial=5
   string "lazywal-desktops:[{"index":0,"
"`,
	} {
		if ds, err := parseSnapshot(r); err == nil {
			t.Errorf("parseSnapshot(%q) = %+v, want an error", r, ds)
		}
	}
}

func TestSetScriptEscaping(t *testing.T) {
	assertPlays(t, setScript(`file:///home/u/it's "quoted".mp4`), `file:///home/u/it's "quoted".mp4`)
}

func TestSet(t *testing.T) {
	f := newFake()
	st := shelltest.NewStore()
	v := backendtest.Video(t)
	if err := newPlasma(t, f, st, true).Set(v); err != nil {
		t.Fatal(err)
	}
	if len(f.Calls) != 2 || f.Calls[0].String() != snapshotCmd.String() {
		t.Fatalf("calls = %q, want the snapshot script then the set script", f.Commands())
	}
	assertPlays(t, scriptOf(t, f.Calls[1]), v.URI())

	var saved []desktop
	if err := json.Unmarshal([]byte(st.Get(stateKey)), &saved); err != nil {
		t.Fatalf("state %q: %v", st.Get(stateKey), err)
	}
	assertDesktops(t, saved, replyDesktops(t))
}

func TestSetSavesDesktopsOnlyTheFirstTime(t *testing.T) {
	f := newFake()
	st := shelltest.NewStore()
	v := backendtest.Video(t)
	if err := newPlasma(t, f, st, true).Set(v); err != nil {
		t.Fatal(err)
	}
	first := st.Get(stateKey)
	// What a second Set finds is lazywal's own wallpaper on every desktop.
	f.Outputs[snapshotCmd.String()] = `method return time=2 sender=:1.27 -> destination=:1.313 serial=4300 reply_serial=2
   string "lazywal-desktops:[{"index":0,"wallpaperPlugin":"luisbocanegra.smart.video.wallpaper.reborn","config":{}}]"`
	if err := newPlasma(t, f, st, true).Set(v); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(snapshotCmd.String()); n != 1 {
		t.Errorf("snapshot ran %d times, want once", n)
	}
	if got := st.Get(stateKey); got != first {
		t.Errorf("state = %s, want the first snapshot %s", got, first)
	}
}

func TestSetWithoutSnapshotChangesNothing(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(f *shelltest.Fake)
	}{
		{"plasmashell unreachable", func(f *shelltest.Fake) {
			f.Errors["dbus-send"] = errors.New("org.kde.plasmashell was not provided")
		}},
		{"unexpected reply", func(f *shelltest.Fake) {
			f.Outputs[snapshotCmd.String()] = "method return time=1 sender=:1.2 -> destination=:1.3 serial=4 reply_serial=5\n   string \"\""
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			tc.setup(f)
			st := shelltest.NewStore()
			if err := newPlasma(t, f, st, true).Set(backendtest.Video(t)); err == nil {
				t.Fatal("Set() = nil, want an error")
			}
			if got := f.Commands(); len(got) != 1 || got[0] != snapshotCmd.String() {
				t.Errorf("calls = %q, want only the snapshot script", got)
			}
			if got := st.Get(stateKey); got != "" {
				t.Errorf("state = %q, want nothing saved", got)
			}
		})
	}
}

func TestSetReportsPlasmashellFailure(t *testing.T) {
	f := newFake()
	v := backendtest.Video(t)
	set := evaluateScript(setScript(v.URI()))
	f.Errors[set.String()] = errors.New("org.kde.plasmashell was not provided")
	if err := newPlasma(t, f, shelltest.NewStore(), true).Set(v); err == nil {
		t.Error("Set() = nil, want the dbus-send error")
	}
}

func TestClearRestoresEachDesktop(t *testing.T) {
	f := newFake()
	st := shelltest.NewStore()
	if err := newPlasma(t, f, st, true).Set(backendtest.Video(t)); err != nil {
		t.Fatal(err)
	}
	f.Reset()
	// A later lazywal process clears.
	if err := newPlasma(t, f, st, true).Clear(); err != nil {
		t.Fatal(err)
	}
	script := evaluated(t, f)

	saved := savedIn(t, script)
	want := replyDesktops(t)
	assertDesktops(t, []desktop{saved["0"], saved["1"], saved["2"]}, want)
	if len(saved) != len(want) {
		t.Errorf("saved has %d desktops, want %d", len(saved), len(want))
	}
	for _, line := range []string{
		// Only desktops still showing the video plugin are touched.
		"if (d.wallpaperPlugin !== " + jsString(Plugin) + ") {\n    continue;",
		// Desktops without a saved plugin get the image plugin.
		"if (s === undefined) {\n    d.wallpaperPlugin = " + jsString(imagePlugin) + ";",
		"d.wallpaperPlugin = s.wallpaperPlugin;",
		"d.currentConfigGroup = [\"Wallpaper\", " + jsString(Plugin) + ", \"General\"];",
		"d.writeConfig(k, s.config[k]);",
	} {
		if !strings.Contains(script, line) {
			t.Errorf("script lacks %q:\n%s", line, script)
		}
	}
	if got := st.Get(stateKey); got != restored {
		t.Errorf("state = %q, want %q", got, restored)
	}

	f.Reset()
	if err := newPlasma(t, f, st, true).Clear(); err != nil {
		t.Fatal(err)
	}
	if len(f.Calls) != 0 {
		t.Errorf("second Clear ran %q, want nothing: it would switch a restored video wallpaper to the image plugin", f.Commands())
	}
}

func TestRestoreScriptEmbedsValuesAsJSON(t *testing.T) {
	hostile := "\"; d.wallpaperPlugin = \"x\"; //\u2028</script>"
	ds := []desktop{
		{Index: 0, Plugin: hostile},
		{Index: 3, Plugin: Plugin, Config: map[string]json.RawMessage{"LastVideo": json.RawMessage(jsString(hostile))}},
		{Index: -1, Plugin: "org.kde.image"},
		{Index: 4},
	}
	saved := savedIn(t, restoreScript(ds))
	// Entries without a usable index or plugin are left to the fallback.
	assertDesktops(t, []desktop{saved["0"], saved["3"]}, ds[:2])
	if len(saved) != 2 {
		t.Errorf("saved = %+v, want only desktops 0 and 3", saved)
	}
}

func TestClearFailureKeepsSnapshot(t *testing.T) {
	f := newFake()
	st := shelltest.NewStore()
	if err := newPlasma(t, f, st, true).Set(backendtest.Video(t)); err != nil {
		t.Fatal(err)
	}
	snapshot := st.Get(stateKey)
	f.Errors["dbus-send"] = errors.New("org.kde.plasmashell was not provided")
	if err := newPlasma(t, f, st, true).Clear(); err == nil {
		t.Fatal("Clear() = nil, want the dbus-send error")
	}
	if got := st.Get(stateKey); got != snapshot {
		t.Fatalf("state = %q after a failed Clear, want the snapshot kept", got)
	}

	delete(f.Errors, "dbus-send")
	f.Reset()
	if err := newPlasma(t, f, st, true).Clear(); err != nil {
		t.Fatal(err)
	}
	assertDesktops(t, []desktop{savedIn(t, evaluated(t, f))["1"]}, replyDesktops(t)[1:2])
}

func TestSetAfterClearSavesAgain(t *testing.T) {
	f := newFake()
	st := shelltest.NewStore()
	p := newPlasma(t, f, st, true)
	for _, step := range []func() error{
		func() error { return p.Set(backendtest.Video(t)) },
		p.Clear,
		func() error { return p.Set(backendtest.Video(t)) },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.Count(snapshotCmd.String()); n != 2 {
		t.Errorf("snapshot ran %d times, want once per Set after a Clear", n)
	}
	if got := st.Get(stateKey); got == "" || got == restored {
		t.Errorf("state = %q, want a new snapshot", got)
	}
}

func TestClearUnreadableSnapshotFallsBack(t *testing.T) {
	f := shelltest.New()
	st := shelltest.NewStore()
	st.Set(stateKey, "{not json")
	if err := newPlasma(t, f, st, true).Clear(); err != nil {
		t.Fatal(err)
	}
	if saved := savedIn(t, evaluated(t, f)); len(saved) != 0 {
		t.Errorf("saved = %+v, want none so every desktop takes the fallback", saved)
	}
	if got := st.Get(stateKey); got != restored {
		t.Errorf("state = %q, want %q", got, restored)
	}
}

func TestClearWithoutSnapshotOnlyTouchesOurPlugin(t *testing.T) {
	f := shelltest.New()
	st := shelltest.NewStore()
	if err := newPlasma(t, f, st, true).Clear(); err != nil {
		t.Fatal(err)
	}
	script := evaluated(t, f)

	guard := "if (d.wallpaperPlugin === " + jsString(Plugin) + ") {"
	i := strings.Index(script, guard)
	if i < 0 {
		t.Fatalf("script does not check for our plugin:\n%s", script)
	}
	body, _, ok := strings.Cut(script[i+len(guard):], "}")
	if want := "d.wallpaperPlugin = " + jsString(imagePlugin) + ";"; !ok || strings.TrimSpace(body) != want {
		t.Errorf("guarded body = %q, want %q", body, want)
	}
	if n := len(regexp.MustCompile(`wallpaperPlugin\s*=[^=]`).FindAllString(script, -1)); n != 1 {
		t.Errorf("script assigns wallpaperPlugin %d times, want only the guarded one:\n%s", n, script)
	}
	if strings.Contains(script, "writeConfig") {
		t.Errorf("Clear rewrites wallpaper config:\n%s", script)
	}
	if got := st.Get(stateKey); got != "" {
		t.Errorf("state = %q, want nothing recorded by the fallback", got)
	}
}
