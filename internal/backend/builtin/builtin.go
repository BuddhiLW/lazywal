// Package builtin registers lazywal's built-in backends and the rules that
// select them. Adding a backend means adding a Register call here (or a
// JSON entry in the user's backends.json); the selection code never changes.
package builtin

import (
	"github.com/BuddhiLW/lazywal/internal/backend/gnome"
	"github.com/BuddhiLW/lazywal/internal/backend/plasma"
	"github.com/BuddhiLW/lazywal/internal/backend/wlr"
	"github.com/BuddhiLW/lazywal/internal/backend/x11"
	"github.com/BuddhiLW/lazywal/internal/shell"
	w "github.com/BuddhiLW/lazywal/internal/wallpaper"
)

// Deps are the ports every built-in backend is wired to.
type Deps struct {
	Runner shell.Runner
	Store  shell.Store
	Logf   func(format string, args ...any)
}

// Priorities: a desktop's native integration beats a generic X11 or Wayland
// fallback. User-defined backends can outrank any of these.
const (
	PriorityNative   = 100 // GNOME Shell extension, Plasma plugin
	PriorityDesktop  = 80  // X11 desktops whose desktop-icons layer we manage
	PriorityFallback = 50  // X11 desktops with panels, no layer handling
	PriorityWayland  = 10  // any wlr-layer-shell compositor
	PriorityX11      = 0   // bare X11 window managers
)

// Desktops that add "GNOME" to XDG_CURRENT_DESKTOP without running GNOME
// Shell, which the Hanabi extension needs.
var gnomeLookalikes = []string{"budgie", "gnome-flashback", "unity", "pantheon"}

// Register adds every built-in backend to reg.
func Register(reg *w.Registry, d Deps) {
	reg.Register(gnome.New(d.Runner, d.Store, d.Logf),
		w.Rule{Desktops: []string{"gnome", "gnome-classic"}, Exclude: gnomeLookalikes, Priority: PriorityNative})

	reg.Register(plasma.New(d.Runner, d.Store, d.Logf),
		w.Rule{Desktops: []string{"kde", "plasma"}, Priority: PriorityNative})

	reg.Register(wlr.NewMpvpaper(d.Runner, d.Store),
		// Mutter (GNOME) has no wlr-layer-shell; KWin, wlroots, Smithay and
		// labwc-based desktops (incl. Budgie 10.10) do.
		w.Rule{Session: w.Wayland, Exclude: []string{"gnome"}, Priority: PriorityWayland},
		w.Rule{Session: w.Wayland, Desktops: []string{"budgie"}, Priority: PriorityWayland})

	xwinwrap := func(name, description string, mode x11.WindowMode) *x11.Xwinwrap {
		return x11.NewXwinwrap(x11.XwinwrapConfig{
			Name: name, Description: description, Mode: mode,
			Runner: d.Runner, Store: d.Store, Logf: d.Logf,
		})
	}

	reg.Register(xwinwrap("xwinwrap", "X11 window managers (xmonad, i3, bspwm, awesome...) via xwinwrap + mpv", x11.Unmanaged),
		w.Rule{Session: w.X11, Priority: PriorityX11})

	reg.Register(xwinwrap("x11-desktop", "X11 desktops with panels (GNOME/KDE without their plugin, Budgie...) via xwinwrap + mpv", x11.DesktopWindow),
		w.Rule{
			Session:  w.X11,
			Desktops: []string{"gnome", "gnome-classic", "gnome-flashback", "kde", "plasma", "budgie", "unity", "pantheon", "deepin", "ukui"},
			Priority: PriorityFallback,
		})

	desktops := []struct {
		name, description string
		desktops          []string
		layer             x11.Layer
	}{
		{"mate", "MATE (X11); hides Caja desktop icons while playing", []string{"mate"}, x11.MateLayer(d.Runner)},
		{"cinnamon", "Cinnamon (X11); hides Nemo desktop icons while playing", []string{"x-cinnamon", "cinnamon"}, x11.CinnamonLayer(d.Runner)},
		{"xfce", "Xfce (X11); stops xfdesktop while playing", []string{"xfce"}, x11.XfceLayer(d.Runner)},
		{"lxqt", "LXQt (X11); turns off the pcmanfm-qt desktop while playing", []string{"lxqt"}, x11.LXQtLayer(d.Runner)},
		{"lxde", "LXDE (X11); turns off the pcmanfm desktop while playing", []string{"lxde"}, x11.LXDELayer(d.Runner)},
	}
	for _, de := range desktops {
		inner := xwinwrap(de.name, de.description, x11.DesktopWindow)
		reg.Register(x11.NewDesktop(inner, de.name, de.description, de.layer, d.Store, d.Logf),
			w.Rule{Session: w.X11, Desktops: de.desktops, Priority: PriorityDesktop})
	}
}
