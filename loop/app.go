package loop

import (
	"fmt"
	"log"
	"os"

	"github.com/BuddhiLW/lazywal/internal/backend/builtin"
	"github.com/BuddhiLW/lazywal/internal/backend/custom"
	"github.com/BuddhiLW/lazywal/internal/backend/x11"
	"github.com/BuddhiLW/lazywal/internal/colors"
	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/rwxrob/bonzai/persisters/injson"
)

// EnvBackend names a backend to use instead of auto-selecting one.
const EnvBackend = "LAZYWAL_BACKEND"

// app is what the commands operate on: the wallpaper service and the color
// generator, wired to real adapters.
type app struct {
	Service *wallpaper.Service
	Colors  *colors.Pywal
}

// newApp is the composition root. Commands call it (instead of using
// package-level state) so nothing touches the system until a command runs,
// and tests can replace it.
var newApp = func() (*app, error) {
	runner := shell.Exec{Log: log.Default()}
	// Stored in ~/.local/state/lazywal/state.json
	store := injson.NewUserState("lazywal", "state.json")

	reg := wallpaper.NewRegistry()
	builtin.Register(reg, builtin.Deps{Runner: runner, Store: store, Logf: log.Printf})

	// User-defined backends extend (or override) the built-ins.
	path := custom.DefaultPath()
	specs, err := custom.Load(path)
	if err != nil {
		return nil, err
	}
	if err := custom.Register(reg, specs, runner, store); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	adoptLegacyState(store)
	keepRetiredReachable(reg, runner, store)

	return &app{
		Service: &wallpaper.Service{
			Registry: reg,
			State:    store,
			Session:  wallpaper.DetectSession(os.Getenv),
			Override: os.Getenv(EnvBackend),
			Logf:     log.Printf,
		},
		Colors: colors.New(runner),
	}, nil
}

// adoptLegacyState attributes a wallpaper started by lazywal <= v1.4.3, which
// tracked xwinwrap PIDs but never recorded a backend, to the xwinwrap
// backend, so switching to any other backend (or clearing) stops it.
func adoptLegacyState(store shell.Store) {
	if store.Get(wallpaper.StateBackend) == "" && store.Get(x11.PIDsKey) != "" {
		store.Set(wallpaper.StateBackend, "xwinwrap")
	}
}

// keepRetiredReachable registers a stand-in for a custom backend that set
// the wallpaper but has since been removed from backends.json, so its
// players can still be stopped.
func keepRetiredReachable(reg *wallpaper.Registry, runner shell.Runner, store shell.Store) {
	if prev := store.Get(wallpaper.StateBackend); prev != "" && !reg.Has(prev) && custom.HasPlayers(prev, store) {
		reg.Register(custom.Retired(prev, runner, store))
	}
}
