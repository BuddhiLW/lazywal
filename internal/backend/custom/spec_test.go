package custom

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

const example = "testdata/backends.json"

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "backends.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// valid returns a spec using every field, for tests to break one at a time.
func valid() Spec {
	return Spec{
		Name:        "demo",
		Description: "demo backend",
		Requires:    []string{"mpv"},
		Rules:       []wallpaper.Rule{{Session: wallpaper.X11, Desktops: []string{"openbox"}, Exclude: []string{"gnome"}, Priority: 5}},
		Set:         [][]string{{"notify-send", "lazywal", "playing {{path}}"}},
		Spawn:       [][]string{{"xwinwrap", "-fs", "--", "mpv", "-wid", "WID", "--", "{{path}}"}},
		Clear:       [][]string{{"notify-send", "lazywal", "cleared"}},
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("HOME", "/home/me")
	for _, tt := range []struct{ xdg, want string }{
		{"/cfg", "/cfg/lazywal/backends.json"},
		{"", "/home/me/.config/lazywal/backends.json"},
		{"relative", "/home/me/.config/lazywal/backends.json"},
	} {
		t.Setenv("XDG_CONFIG_HOME", tt.xdg)
		if got := DefaultPath(); got != tt.want {
			t.Errorf("XDG_CONFIG_HOME=%q: DefaultPath() = %q, want %q", tt.xdg, got, tt.want)
		}
	}
}

func TestLoadExample(t *testing.T) {
	specs, err := Load(example)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range specs {
		names = append(names, s.Name)
	}
	if want := []string{"hyprland-dp1", "hanabi", "xwinwrap-hwdec"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %q, want %q", names, want)
	}
	want := []wallpaper.Rule{{Session: wallpaper.Wayland, Desktops: []string{"hyprland"}, Priority: 200}}
	if !reflect.DeepEqual(specs[0].Rules, want) {
		t.Errorf("rules = %+v, want %+v", specs[0].Rules, want)
	}
}

func TestLoadRoundTrip(t *testing.T) {
	other := valid()
	other.Name, other.Rules, other.Set = "other", nil, nil
	file := File{Backends: []Spec{valid(), other}}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Load(writeConfig(t, string(data)))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, file.Backends) {
		t.Errorf("Load() = %+v\nwant     %+v", got, file.Backends)
	}
}

func TestLoadWithoutConfig(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "missing.json")} {
		specs, err := Load(path)
		if specs != nil || err != nil {
			t.Errorf("Load(%q) = %v, %v; want nil, nil", path, specs, err)
		}
	}
}

func TestLoadErrorsNameFileAndBackend(t *testing.T) {
	tests := []struct {
		name, config string
		want         []string
	}{
		{"unknown backend field", `{"backends": [{"name": "x", "description": "d", "sett": [["a"]]}]}`,
			[]string{`"x" (backend #1)`, `unknown field "sett"`}},
		{"unknown rule field", `{"backends": [{"name": "x", "description": "d", "spawn": [["a"]], "rules": [{"sesion": "x11"}]}]}`,
			[]string{`"x" (backend #1)`, `unknown field "sesion"`}},
		{"unknown top-level field", `{"backend": []}`,
			[]string{`unknown field "backend"`}},
		{"wrong type", `{"backends": [{"name": "x", "description": "d", "spawn": "mpvpaper"}]}`,
			[]string{`"x" (backend #1)`, "spawn"}},
		{"syntax error", "{\"backends\": [\n  {\"name\": \"x\",}\n]}",
			[]string{"invalid JSON on line 2"}},
		{"not an object", `[]`,
			[]string{`want {"backends": [...]}, found array on line 1`}},
		{"backends not a list", "{\n\"backends\": {}}",
			[]string{`want {"backends": [...]}, found object on line 2`}},
		{"trailing data", `{"backends": []} {}`,
			[]string{"unexpected data"}},
		{"empty file", "",
			[]string{"empty file"}},
		{"invalid spec", `{"backends": [{"name": "a", "description": "d", "spawn": [["a"]]}, {"name": "b", "spawn": [["b"]]}]}`,
			[]string{`"b" (backend #2)`, "description is empty"}},
		{"nameless spec", `{"backends": [{"description": "d", "spawn": [["a"]]}]}`,
			[]string{"backend #1: name is empty"}},
		{"duplicate name", `{"backends": [{"name": "a", "description": "d", "spawn": [["a"]]}, {"name": "a", "description": "e", "set": [["b"]]}]}`,
			[]string{`"a" (backend #2)`, "already used by backend #1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.config)
			specs, err := Load(path)
			if err == nil {
				t.Fatalf("Load() = %+v, want an error", specs)
			}
			for _, want := range append(tt.want, path) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatalf("valid spec: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*Spec)
		want   []string
	}{
		{"empty name", func(s *Spec) { s.Name = "" }, []string{"name is empty"}},
		{"upper-case name", func(s *Spec) { s.Name = "Demo" }, []string{`name "Demo" must be lower-case with no spaces`}},
		{"name with space", func(s *Spec) { s.Name = "my demo" }, []string{"lower-case with no spaces"}},
		{"name with tab", func(s *Spec) { s.Name = "my\tdemo" }, []string{"lower-case with no spaces"}},
		{"no description", func(s *Spec) { s.Description = " " }, []string{"description is empty"}},
		{"empty requirement", func(s *Spec) { s.Requires = []string{""} }, []string{"requires[0] is empty"}},
		{"unknown session", func(s *Spec) { s.Rules[0].Session = "Wayland" }, []string{`rules[0]: unknown session "Wayland"`}},
		{"no set or spawn", func(s *Spec) { s.Set, s.Spawn = nil, nil }, []string{`at least one "set" or "spawn"`}},
		{"empty command", func(s *Spec) { s.Spawn = [][]string{{}} }, []string{"spawn[0]: no program given"}},
		{"empty program", func(s *Spec) { s.Clear = [][]string{{"", "x"}} }, []string{"clear[0]: no program given"}},
		{"unknown placeholder", func(s *Spec) { s.Set[0][2] = "{{file}}" }, []string{`set[0]: argument "{{file}}": unknown placeholder`}},
		{"misspelt placeholder", func(s *Spec) { s.Spawn[0][7] = "{{path}" }, []string{"spawn[0]", "unknown placeholder"}},
		{"placeholder in program", func(s *Spec) { s.Spawn[0][0] = "{{path}}" }, []string{"spawn[0]: program", "cannot contain a placeholder"}},
		{"placeholder in clear", func(s *Spec) { s.Clear[0][2] = "{{uri}}" }, []string{"clear[0]", "no video"}},
		{"every problem at once", func(s *Spec) { s.Name, s.Set, s.Spawn = "", nil, nil },
			[]string{"name is empty", `at least one "set" or "spawn"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := valid()
			tt.mutate(&s)
			err := s.Validate()
			if err == nil {
				t.Fatal("Validate() = nil, want an error")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}
