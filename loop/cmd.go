// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

package loop

import (
	"fmt"

	dependencies "github.com/BuddhiLW/lazywal/internal/check"
	"github.com/rwxrob/bonzai"
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
	Vers:  `v1.3.0`,

	Cmds: []*bonzai.Cmd{
		HelpCmd,
		LoopCmd, ClearCmd, PywalCmd, dependencies.TestDepsCmd,
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

* lazywal set <path>                    (Auto-detects monitor configuration)
* lazywal set <path> display <WxH>      (Manual display size - e.g. 1440x1080)
* lazywal kill                          (Kills all xwinwrap processes)
* lazywal pywal                         (Updates color scheme using pywal)

Features:
* Multi-monitor support with correct positioning
* Automatic monitor detection
* Pywal integration for system-wide color schemes
* Handles video files and animated GIFs

Dependencies:
* xwinwrap - For window creation
* mpv     - For video playback
* xrandr  - For monitor detection
* pywal   - Optional, for color scheme generation

See the README.md for more information and examples.
`,
}
