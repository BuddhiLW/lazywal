package gnome_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/backend/gnome"
	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

const (
	extensionDep = "Hanabi GNOME extension"
	schemaDep    = "Hanabi settings schema"

	// stateKey is part of lazywal's persisted state format.
	stateKey = "lazywal_gnome_saved"

	// The user's own Hanabi settings, as gsettings get prints them.
	userVideo     = `'/home/u/Videos/mine.mp4'`
	userSlideshow = "true"
)

// gnome-extensions info output on GNOME 45+.
const (
	infoEnabled = `hanabi-extension@jeffshee.github.io
  Name: Hanabi Extension
  Description: Live Wallpaper for GNOME
  Path: /home/u/.local/share/gnome-shell/extensions/hanabi-extension@jeffshee.github.io
  URL: https://github.com/jeffshee/gnome-ext-hanabi
  Version: 1
  Enabled: Yes
  State: ACTIVE`
	infoDisabled = `hanabi-extension@jeffshee.github.io
  Name: Hanabi Extension
  Description: Live Wallpaper for GNOME
  Path: /home/u/.local/share/gnome-shell/extensions/hanabi-extension@jeffshee.github.io
  URL: https://github.com/jeffshee/gnome-ext-hanabi
  Version: 1
  Enabled: No
  State: INACTIVE`
)

var (
	userDir = "/home/u/.local/share/gnome-shell/extensions/" + gnome.UUID + "/schemas"
	sysDir  = "/usr/share/gnome-shell/extensions/" + gnome.UUID + "/schemas"

	info    = shell.Command("gnome-extensions", "info", gnome.UUID)
	enable  = shell.Command("gnome-extensions", "enable", gnome.UUID)
	disable = shell.Command("gnome-extensions", "disable", gnome.UUID)
)

func get(key string) shell.Cmd { return shell.Command("gsettings", "get", gnome.Schema, key) }

func set(key, value string) shell.Cmd {
	return shell.Command("gsettings", "set", gnome.Schema, key, value)
}

// newFake scripts an installed Hanabi showing the user's own video with its
// slideshow on; infoOut says whether the extension is enabled.
func newFake(infoOut string) *shelltest.Fake {
	f := shelltest.New()
	f.Outputs[info.String()] = infoOut
	f.Outputs[get("video-path").String()] = userVideo
	f.Outputs[get("change-wallpaper").String()] = userSlideshow
	return f
}

func newHanabi(t *testing.T, f *shelltest.Fake, st shell.Store) *gnome.Hanabi {
	h := gnome.New(f, st, t.Logf)
	h.SchemaDirs = []string{userDir, sysDir}
	return h
}

func listKeys(dir string) shell.Cmd {
	if dir == "" {
		return shell.Command("gsettings", "list-keys", gnome.Schema)
	}
	return shell.Command("gsettings", "--schemadir", dir, "list-keys", gnome.Schema)
}

// failAll makes every schema lookup fail, as when Hanabi's schema is not installed.
func failAll(f *shelltest.Fake) {
	for _, dir := range []string{"", userDir, sysDir} {
		c := listKeys(dir)
		f.Errors[c.String()] = shelltest.Exit(c, 1)
	}
}

func assertCalls(t *testing.T, f *shelltest.Fake, want ...shell.Cmd) {
	t.Helper()
	if !reflect.DeepEqual(f.Calls, want) {
		t.Errorf("calls =\n%q\nwant\n%q", f.Commands(), want)
	}
}

func TestContract(t *testing.T) {
	backendtest.ContractWithRunner(t, func(t *testing.T, r *shelltest.Fake, s *shelltest.Store) wallpaper.Backend {
		for k, v := range newFake(infoDisabled).Outputs {
			r.Outputs[k] = v
		}
		return newHanabi(t, r, s)
	})
}

func TestNewDefaultSchemaDirs(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	h := gnome.New(shelltest.New(), shelltest.NewStore(), nil)
	if want := []string{userDir, sysDir}; !reflect.DeepEqual(h.SchemaDirs, want) {
		t.Errorf("SchemaDirs = %q, want %q", h.SchemaDirs, want)
	}
}

func TestCheckListsBothMissingPrograms(t *testing.T) {
	f := shelltest.New()
	f.Missing["gnome-extensions"] = true
	f.Missing["gsettings"] = true
	backendtest.MissingDeps(t, newHanabi(t, f, shelltest.NewStore()).Check(), "gnome-extensions", "gsettings")
	if len(f.Calls) != 0 {
		t.Errorf("Check ran %q without the programs to run", f.Commands())
	}
}

func TestCheckExtensionNotInstalled(t *testing.T) {
	f := shelltest.New()
	f.Errors[info.String()] = shelltest.Exit(info, 2)
	backendtest.MissingDeps(t, newHanabi(t, f, shelltest.NewStore()).Check(), extensionDep)
}

func TestCheckExtensionAndGsettingsMissing(t *testing.T) {
	f := shelltest.New()
	f.Missing["gsettings"] = true
	f.Errors[info.String()] = shelltest.Exit(info, 2)
	backendtest.MissingDeps(t, newHanabi(t, f, shelltest.NewStore()).Check(), "gsettings", extensionDep)
}

func TestCheckSchemaUnreachable(t *testing.T) {
	f := shelltest.New()
	failAll(f)
	backendtest.MissingDeps(t, newHanabi(t, f, shelltest.NewStore()).Check(), schemaDep)
}

func TestCheckFindsSchemaInExtensionDir(t *testing.T) {
	f := shelltest.New()
	failAll(f)
	delete(f.Errors, listKeys(sysDir).String())
	if err := newHanabi(t, f, shelltest.NewStore()).Check(); err != nil {
		t.Errorf("Check() = %v, want nil", err)
	}
}

func TestSetSavesUserStateThenPlays(t *testing.T) {
	f := newFake(infoEnabled)
	v := backendtest.Video(t)
	if err := newHanabi(t, f, shelltest.NewStore()).Set(v); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, f,
		listKeys(""),
		info,
		get("video-path"),
		get("change-wallpaper"),
		set("video-path", `'`+filepath.Dir(v.Path())+`/it\'s a "video".mp4'`),
		set("change-wallpaper", "false"),
		enable,
	)
}

func TestSetQuotesPathAsGVariant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, `it's a \ "video".mp4`)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := wallpaper.NewVideo(path)
	if err != nil {
		t.Fatal(err)
	}
	f := newFake(infoEnabled)
	if err := newHanabi(t, f, shelltest.NewStore()).Set(v); err != nil {
		t.Fatal(err)
	}
	// Backslash and single quote escaped, everything else verbatim.
	want := set("video-path", `'`+dir+`/it\'s a \\ "video".mp4'`)
	if !f.Ran(want.String()) {
		t.Errorf("missing %q in %q", want, f.Commands())
	}
}

func TestSetSavesOnlyTheFirstTime(t *testing.T) {
	st := shelltest.NewStore()
	f := newFake(infoDisabled)
	v := backendtest.Video(t)
	if err := newHanabi(t, f, st).Set(v); err != nil {
		t.Fatal(err)
	}
	// What a second Set finds is lazywal's own wallpaper, not the user's.
	f.Outputs[info.String()] = infoEnabled
	f.Outputs[get("video-path").String()] = `'` + v.Path() + `'`
	f.Outputs[get("change-wallpaper").String()] = "false"
	if err := newHanabi(t, f, st).Set(v); err != nil {
		t.Fatal(err)
	}
	for _, c := range []shell.Cmd{info, get("video-path"), get("change-wallpaper")} {
		if n := f.Count(c.String()); n != 1 {
			t.Errorf("%s ran %d times, want once", c, n)
		}
	}

	f.Reset()
	if err := newHanabi(t, f, st).Clear(); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, f, listKeys(""), disable, set("video-path", userVideo), set("change-wallpaper", userSlideshow))
}

func TestClearLeavesEnabledHanabiEnabled(t *testing.T) {
	st := shelltest.NewStore()
	f := newFake(infoEnabled)
	if err := newHanabi(t, f, st).Set(backendtest.Video(t)); err != nil {
		t.Fatal(err)
	}
	f.Reset()
	// A later lazywal process clears.
	if err := newHanabi(t, f, st).Clear(); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, f, listKeys(""), set("video-path", userVideo), set("change-wallpaper", userSlideshow))

	f.Reset()
	if err := newHanabi(t, f, st).Clear(); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, f)
}

func TestClearFollowsWhetherHanabiWasEnabled(t *testing.T) {
	for _, tc := range []struct {
		name, info string
		enabled    bool
	}{
		{"GNOME 45+ enabled", infoEnabled, true},
		{"GNOME 45+ disabled", infoDisabled, false},
		{"GNOME 45+ enabled with an error", "  Enabled: Yes\n  State: ERROR", true},
		{"GNOME 40 enabled", "  Name: Hanabi\n  State: ENABLED", true},
		{"GNOME 40 disabled", "  Name: Hanabi\n  State: DISABLED", false},
		{"translated enabled", "  Aktiviert: Ja\n  Status: ACTIVE", true},
		{"translated disabled", "  Aktiviert: Nein\n  Status: INACTIVE", false},
		{"unreadable", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := shelltest.NewStore()
			f := newFake(tc.info)
			if err := newHanabi(t, f, st).Set(backendtest.Video(t)); err != nil {
				t.Fatal(err)
			}
			if err := newHanabi(t, f, st).Clear(); err != nil {
				t.Fatal(err)
			}
			if disabled := f.Ran(disable.String()); disabled == tc.enabled {
				t.Errorf("Clear disabled the extension: %v, want %v", disabled, !tc.enabled)
			}
		})
	}
}

func TestClearWithNothingSavedRunsNothing(t *testing.T) {
	f := shelltest.New()
	f.Missing["gnome-extensions"] = true
	f.Missing["gsettings"] = true
	f.Errors["gnome-extensions"] = shelltest.Exit(disable, 2)
	f.Errors["gsettings"] = shelltest.Exit(listKeys(""), 1)
	h := newHanabi(t, f, shelltest.NewStore())
	for range 2 {
		if err := h.Clear(); err != nil {
			t.Fatalf("Clear() = %v, want nil without Hanabi installed", err)
		}
	}
	assertCalls(t, f)
}

func TestClearFailureKeepsStateForRetry(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail func(f *shelltest.Fake)
	}{
		{"schema unreachable", failAll},
		{"disable", func(f *shelltest.Fake) { f.Errors[disable.String()] = shelltest.Exit(disable, 2) }},
		{"video-path", func(f *shelltest.Fake) {
			c := set("video-path", userVideo)
			f.Errors[c.String()] = shelltest.Exit(c, 1)
		}},
		{"change-wallpaper", func(f *shelltest.Fake) {
			c := set("change-wallpaper", userSlideshow)
			f.Errors[c.String()] = shelltest.Exit(c, 1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := shelltest.NewStore()
			f := newFake(infoDisabled)
			if err := newHanabi(t, f, st).Set(backendtest.Video(t)); err != nil {
				t.Fatal(err)
			}
			tc.fail(f)
			if err := newHanabi(t, f, st).Clear(); err == nil {
				t.Fatal("Clear() = nil, want the failed command's error")
			}

			clear(f.Errors)
			f.Reset()
			if err := newHanabi(t, f, st).Clear(); err != nil {
				t.Fatal(err)
			}
			assertCalls(t, f, listKeys(""), disable, set("video-path", userVideo), set("change-wallpaper", userSlideshow))
		})
	}
}

func TestClearUnreadableStateDisables(t *testing.T) {
	st := shelltest.NewStore()
	st.Set(stateKey, "{not json")
	f := shelltest.New()
	if err := newHanabi(t, f, st).Clear(); err != nil {
		t.Fatal(err)
	}
	assertCalls(t, f, disable)
	if got := st.Get(stateKey); got != "" {
		t.Errorf("state = %q, want it dropped", got)
	}
}

func TestSetWithoutExtensionChangesNothing(t *testing.T) {
	st := shelltest.NewStore()
	f := newFake(infoEnabled)
	f.Errors[info.String()] = shelltest.Exit(info, 2)
	if err := newHanabi(t, f, st).Set(backendtest.Video(t)); shell.ExitCode(err) != 2 {
		t.Fatalf("Set() = %v, want the gnome-extensions info exit error", err)
	}
	assertCalls(t, f, listKeys(""), info)
	if got := st.Get(stateKey); got != "" {
		t.Errorf("state = %q, want nothing saved", got)
	}
}

func TestSetUsesSchemaDir(t *testing.T) {
	st := shelltest.NewStore()
	f := newFake(infoEnabled)
	failAll(f)
	delete(f.Errors, listKeys(sysDir).String())
	getVideo := shell.Command("gsettings", "--schemadir", sysDir, "get", gnome.Schema, "video-path")
	f.Outputs[getVideo.String()] = userVideo
	getSlideshow := shell.Command("gsettings", "--schemadir", sysDir, "get", gnome.Schema, "change-wallpaper")
	f.Outputs[getSlideshow.String()] = userSlideshow
	v := backendtest.Video(t)
	if err := newHanabi(t, f, st).Set(v); err != nil {
		t.Fatal(err)
	}
	if err := newHanabi(t, f, st).Clear(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []shell.Cmd{
		getVideo,
		shell.Command("gsettings", "--schemadir", sysDir, "set", gnome.Schema, "video-path", `'`+filepath.Dir(v.Path())+`/it\'s a "video".mp4'`),
		shell.Command("gsettings", "--schemadir", sysDir, "set", gnome.Schema, "video-path", userVideo),
	} {
		if !f.Ran(want.String()) {
			t.Errorf("missing %q in %q", want, f.Commands())
		}
	}
}

func TestSetWithoutSchemaDoesNotEnable(t *testing.T) {
	f := newFake(infoEnabled)
	failAll(f)
	if err := newHanabi(t, f, shelltest.NewStore()).Set(backendtest.Video(t)); err == nil {
		t.Fatal("Set() = nil, want error when the schema is unreachable")
	}
	assertCalls(t, f, listKeys(""), listKeys(userDir), listKeys(sysDir))
}

func TestSetStopsAtFirstFailure(t *testing.T) {
	f := newFake(infoEnabled)
	v := backendtest.Video(t)
	c := set("video-path", `'`+filepath.Dir(v.Path())+`/it\'s a "video".mp4'`)
	f.Errors[c.String()] = shelltest.Exit(c, 1)
	if err := newHanabi(t, f, shelltest.NewStore()).Set(v); shell.ExitCode(err) != 1 {
		t.Fatalf("Set() = %v, want the gsettings exit error", err)
	}
	if f.Ran(enable.String()) {
		t.Errorf("enabled the extension after a failed gsettings set: %q", f.Commands())
	}
}
