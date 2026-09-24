package custom

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

// Backend runs the commands of a user's Spec.
type Backend struct {
	spec   Spec
	runner shell.Runner
	procs  shell.Tracker
}

// New validates spec and returns its backend. st remembers the players
// spawned, under a key of the backend's own.
func New(spec Spec, r shell.Runner, st shell.Store) (*Backend, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	spec = clone(spec) // later edits to the caller's slices must not bypass validation
	return &Backend{
		spec:   spec,
		runner: r,
		procs:  shell.Tracker{Runner: r, Store: st, Key: pidsKey(spec.Name)},
	}, nil
}

// Register adds a backend for each spec with the spec's rules. A name
// already registered is replaced, which is how users override a built-in.
// Nothing is registered unless every spec is valid.
func Register(reg *wallpaper.Registry, specs []Spec, r shell.Runner, st shell.Store) error {
	if err := validateAll(specs); err != nil {
		return err
	}
	backends := make([]*Backend, len(specs))
	for i, s := range specs {
		b, err := New(s, r, st)
		if err != nil {
			return err
		}
		backends[i] = b
	}
	for _, b := range backends {
		reg.Register(b, b.spec.Rules...)
	}
	return nil
}

func (b *Backend) Name() string     { return b.spec.Name }
func (b *Backend) Describe() string { return b.spec.Description }

// Check requires the listed programs and the program of every command.
func (b *Backend) Check() error {
	req := wallpaper.Require(b.runner.LookPath)
	req.Programs(b.programs()...)
	return req.Err()
}

func (b *Backend) programs() []string {
	var names []string
	add := func(name string) {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	for _, p := range b.spec.Requires {
		add(p)
	}
	for _, group := range [][][]string{b.spec.Set, b.spec.Spawn, b.spec.Clear} {
		for _, argv := range group {
			add(argv[0])
		}
	}
	return names
}

// Set stops the players it spawned before, runs the set commands in order
// and then spawns the players. It stops at the first command that fails.
func (b *Backend) Set(v wallpaper.Video) error {
	b.procs.KillAll()
	for _, argv := range b.spec.Set {
		if err := b.runner.Run(expand(argv, v)); err != nil {
			return err
		}
	}
	for _, argv := range b.spec.Spawn {
		if _, err := b.procs.Spawn(expand(argv, v)); err != nil {
			return err
		}
	}
	return nil
}

// Clear stops the players and runs every clear command, even after one
// fails, so as much as possible is undone; it reports all failures.
func (b *Backend) Clear() error {
	b.procs.KillAll()
	var errs []error
	for _, argv := range b.spec.Clear {
		if err := b.runner.Run(shell.Command(argv[0], argv[1:]...)); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// expand builds the command for argv with the placeholders replaced. Each
// argument stays a single argv element whatever the path contains, and a
// path that happens to contain a placeholder is not expanded again.
func expand(argv []string, v wallpaper.Video) shell.Cmd {
	r := strings.NewReplacer(PathPlaceholder, v.Path(), URIPlaceholder, v.URI())
	args := make([]string, len(argv)-1)
	for i, a := range argv[1:] {
		args[i] = r.Replace(a)
	}
	return shell.Command(argv[0], args...)
}

func clone(s Spec) Spec {
	commands := func(cs [][]string) [][]string {
		out := make([][]string, len(cs))
		for i, c := range cs {
			out[i] = slices.Clone(c)
		}
		return out
	}
	rules := make([]wallpaper.Rule, len(s.Rules))
	for i, r := range s.Rules {
		r.Desktops, r.Exclude = slices.Clone(r.Desktops), slices.Clone(r.Exclude)
		rules[i] = r
	}
	s.Requires, s.Rules = slices.Clone(s.Requires), rules
	s.Set, s.Spawn, s.Clear = commands(s.Set), commands(s.Spawn), commands(s.Clear)
	return s
}

// pidsKey is where a custom backend named name tracks its players.
func pidsKey(name string) string { return "lazywal_custom_" + name + "_pids" }

// Retired stands in for a custom backend whose spec was removed from
// backends.json while its players may still run: it cannot Set, but its
// Clear stops what the removed backend spawned. Register it without rules so
// only the recorded state reaches it.
func Retired(name string, r shell.Runner, st shell.Store) wallpaper.Backend {
	return &retired{name: name, procs: shell.Tracker{Runner: r, Store: st, Key: pidsKey(name)}}
}

type retired struct {
	name  string
	procs shell.Tracker
}

func (b *retired) Name() string { return b.name }
func (b *retired) Describe() string {
	return "removed from backends.json; kept only to stop what it started"
}
func (b *retired) Check() error {
	return &wallpaper.MissingDepsError{Deps: []string{"backends.json entry " + b.name}}
}
func (b *retired) Set(wallpaper.Video) error {
	return fmt.Errorf("backend %s was removed from backends.json", b.name)
}
func (b *retired) Clear() error {
	b.procs.KillAll()
	return nil
}

// HasPlayers reports whether a custom backend named name left tracked players.
func HasPlayers(name string, st shell.Store) bool { return st.Get(pidsKey(name)) != "" }
