// Package custom builds backends from the user's JSON config. It is
// lazywal's Open-Closed extension point: users add backends for desktops
// lazywal does not know, or replace a built-in one, by describing the
// commands to run instead of changing lazywal's code.
package custom

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

// Placeholders expanded in command arguments. Each expands inside its own
// argument, so the path never needs quoting.
const (
	PathPlaceholder = "{{path}}" // absolute path of the video
	URIPlaceholder  = "{{uri}}"  // file:// URL of the video
)

// Spec describes a backend as data. Commands are argv lists run directly,
// never through a shell.
type Spec struct {
	// Name selects the backend (LAZYWAL_BACKEND); reusing a built-in's name
	// replaces that backend.
	Name        string `json:"name"`
	Description string `json:"description"`
	// Requires lists programs needed besides the commands' own programs,
	// which Check requires anyway.
	Requires []string `json:"requires"`
	// Rules select the backend automatically; without any it is only used
	// when named by LAZYWAL_BACKEND.
	Rules []wallpaper.Rule `json:"rules"`
	// Set commands run to completion, in order, on every Set, so they must be
	// safe to repeat.
	Set [][]string `json:"set"`
	// Spawn commands start long-running players. lazywal tracks them and
	// kills them on the next Set or Clear.
	Spawn [][]string `json:"spawn"`
	// Clear commands undo what Set commands changed. They must succeed when
	// nothing is set and cannot use placeholders, as there is no video.
	Clear [][]string `json:"clear"`
}

// File is the layout of the backends config file.
type File struct {
	Backends []Spec `json:"backends"`
}

// DefaultPath is where the backends config lives:
// $XDG_CONFIG_HOME/lazywal/backends.json, else ~/.config/lazywal/backends.json.
// It is empty when neither location is known.
func DefaultPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(dir) { // the XDG spec says to ignore relative paths
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "lazywal", "backends.json")
}

// Load reads and validates the specs in the config file at path. A missing
// file is not an error: it means the user defined no backends. Unknown
// fields are errors so that typos surface instead of being ignored.
func Load(path string) ([]Spec, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading backends config: %w", err)
	}
	specs, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return specs, nil
}

func parse(data []byte) ([]Spec, error) {
	// Decode each backend on its own so errors can name it.
	var raw struct {
		Backends []json.RawMessage `json:"backends"`
	}
	if err := decodeStrict(data, &raw); err != nil {
		return nil, jsonError(data, err)
	}
	specs := make([]Spec, len(raw.Backends))
	for i, msg := range raw.Backends {
		if err := decodeStrict(msg, &specs[i]); err != nil {
			var named struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(msg, &named)
			return nil, fmt.Errorf("%s: %w", label(i, named.Name), err)
		}
	}
	if err := validateAll(specs); err != nil {
		return nil, err
	}
	return specs, nil
}

func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New(`empty file, want {"backends": [...]}`)
		}
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after the top-level object")
	}
	return nil
}

// jsonError turns decoder errors, which carry only a byte offset and Go
// type names, into messages about the user's file.
func jsonError(data []byte, err error) error {
	line := func(offset int64) int {
		return 1 + bytes.Count(data[:min(int(offset), len(data))], []byte("\n"))
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return fmt.Errorf("invalid JSON on line %d: %w", line(syntax.Offset), err)
	}
	var typ *json.UnmarshalTypeError
	if errors.As(err, &typ) {
		return fmt.Errorf(`want {"backends": [...]}, found %s on line %d`, typ.Value, line(typ.Offset))
	}
	return err
}

func label(i int, name string) string {
	if name == "" {
		return "backend #" + strconv.Itoa(i+1)
	}
	return strconv.Quote(name) + " (backend #" + strconv.Itoa(i+1) + ")"
}

// validateAll validates every spec and rejects duplicate names, which would
// otherwise silently replace one another.
func validateAll(specs []Spec) error {
	seen := map[string]int{}
	for i, s := range specs {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("%s: %w", label(i, s.Name), err)
		}
		if j, dup := seen[s.Name]; dup {
			return fmt.Errorf("%s: name already used by backend #%d", label(i, s.Name), j+1)
		}
		seen[s.Name] = i
	}
	return nil
}

// Validate reports every problem with s, so a config can be fixed in one go.
func (s Spec) Validate() error {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	switch {
	case s.Name == "":
		add("name is empty")
	case s.Name != strings.ToLower(s.Name) || strings.IndexFunc(s.Name, unicode.IsSpace) >= 0:
		add("name %q must be lower-case with no spaces", s.Name)
	}
	if strings.TrimSpace(s.Description) == "" {
		add("description is empty")
	}
	for i, p := range s.Requires {
		if strings.TrimSpace(p) == "" {
			add("requires[%d] is empty", i)
		}
	}
	for i, r := range s.Rules {
		if r.Session != wallpaper.AnySession && r.Session != wallpaper.X11 && r.Session != wallpaper.Wayland {
			add("rules[%d]: unknown session %q (want %q, %q or none)", i, r.Session, wallpaper.X11, wallpaper.Wayland)
		}
	}
	if len(s.Set) == 0 && len(s.Spawn) == 0 {
		add(`needs at least one "set" or "spawn" command`)
	}
	for _, group := range []struct {
		field        string
		commands     [][]string
		placeholders bool
	}{
		{"set", s.Set, true},
		{"spawn", s.Spawn, true},
		{"clear", s.Clear, false},
	} {
		for i, argv := range group.commands {
			if problem := checkCommand(argv, group.placeholders); problem != "" {
				add("%s[%d]: %s", group.field, i, problem)
			}
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func checkCommand(argv []string, placeholders bool) string {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return "no program given"
	}
	if hasBraces(argv[0]) {
		return fmt.Sprintf("program %q cannot contain a placeholder", argv[0])
	}
	for _, arg := range argv[1:] {
		if !placeholders && hasBraces(arg) {
			return fmt.Sprintf("argument %q: clear commands have no video to expand", arg)
		}
		if hasBraces(strings.NewReplacer(PathPlaceholder, "", URIPlaceholder, "").Replace(arg)) {
			return fmt.Sprintf("argument %q: unknown placeholder (only %s and %s are expanded)",
				arg, PathPlaceholder, URIPlaceholder)
		}
	}
	return ""
}

// hasBraces catches unknown and misspelt placeholders ("{{file}}", "{{path}").
func hasBraces(s string) bool { return strings.Contains(s, "{{") || strings.Contains(s, "}}") }
