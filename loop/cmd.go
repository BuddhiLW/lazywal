// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

package loop

import (
	"fmt"

	"github.com/BuddhiLW/bonzai"
)

// showHelp displays help for the given command
func showHelp(cmd *bonzai.Cmd) error {
	fmt.Printf("Name: %s\n", cmd.Name)
	if cmd.Short != "" {
		fmt.Printf("Description: %s\n", cmd.Short)
	}
	if cmd.Vers != "" {
		fmt.Printf("Version: %s\n", cmd.Vers)
	}
	if cmd.Usage != "" {
		fmt.Printf("Usage: %s\n", cmd.Usage)
	}
	if len(cmd.Cmds) > 0 {
		fmt.Println("\nCommands:")
		for _, c := range cmd.Cmds {
			if c.Name != "" && !c.IsHidden() {
				fmt.Printf("  %s - %s\n", c.Name, c.Short)
			}
		}
	}
	if cmd.Long != "" {
		fmt.Printf("\n%s\n", cmd.Long)
	}
	return nil
}

// HelpCmd provides basic help functionality
var HelpCmd = &bonzai.Cmd{
	Name:  `help`,
	Alias: `h|?`,
	Short: `display help information`,
	Do: func(x *bonzai.Cmd, args ...string) error {
		caller := x.Caller()
		if caller != nil && caller != x {
			return showHelp(caller)
		}
		// Fallback: show help for self
		return showHelp(x)
	},
}

var Cmd = &bonzai.Cmd{
	Name:  `lazywal`,
	Short: `video/gif wallpaper client`,
	Vers:  version(),

	Cmds: []*bonzai.Cmd{
		HelpCmd,
		LoopCmd, ClearCmd, PywalCmd, BackendsCmd, DepsCmd,
	},

	// Show help when called without args (instead of using Def which causes issues)
	Do: func(x *bonzai.Cmd, args ...string) error {
		return showHelp(x)
	},

	Long: `
Lazywal: a terminal client to help setup video-loops/gifs as background.

Copyright 2021-2024 Zayac-The-Engineer, 2024-2025 Pedro G. Branquinho (Go version)
License: MIT License
Site: buddhilw.com
Source: git@github.com/BuddhiLW/lazywal.git
Issues: github.com/BuddhiLW/lazywal/issues

You can use the following commands:

* lazywal set <path>                    (Auto-detects desktop and monitors)
* lazywal set <path> colors             (Also apply pywal colors)
* lazywal clear                         (Stops the wallpaper, restores the desktop)
* lazywal pywal                         (Updates color scheme using pywal)
* lazywal backends                      (Shows detected session and backends)

Backends (auto-selected; override with LAZYWAL_BACKEND=<name>):
* xwinwrap     - X11 window managers (xmonad, i3, bspwm...)
* mate, cinnamon, xfce, lxqt, lxde - X11 desktops (hide desktop icons while playing)
* x11-desktop  - other X11 desktops with panels
* gnome        - GNOME Wayland/X11, via the Hanabi extension
* plasma       - KDE Plasma 6, via Smart Video Wallpaper Reborn
* mpvpaper     - Hyprland, Sway, river, niri, Wayfire, labwc, KDE, COSMIC...
* your own     - defined in ~/.config/lazywal/backends.json

Features:
* Multi-monitor support with correct positioning
* Pywal integration for system-wide color schemes
* Handles video files and animated GIFs

See the README.md for more information and examples.
`,
}
