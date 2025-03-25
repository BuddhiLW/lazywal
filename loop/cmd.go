// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

package loop

import (
	dependencies "github.com/BuddhiLW/lazywal/internal/check"
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/conf"
	"github.com/rwxrob/help"
	"github.com/rwxrob/vars"
)

func init() {
	if err := Z.Conf.SoftInit(); err != nil {
		panic(err)
	}
	if err := Z.Vars.SoftInit(); err != nil {
		panic(err)
	}
}

var Cmd = &Z.Cmd{
	Name:      `lazywal`,
	Summary:   `Lazywal: a terminal client to facilitate setting up video-loops/gifs as background.`,
	Version:   `v1.2.0`,
	Copyright: `Copyright 2021-2024 Zayac-The-Engineer, 2024-2025 Pedro G. Branquinho (Go version)`,
	License:   `MIT License`,
	Site:      `buddhilw.com`,
	Source:    `git@github.com/BuddhiLW/lazywal.git`,
	Issues:    `github.com/BuddhiLW/lazywal/issues`,

	Commands: []*Z.Cmd{
		// standard external branch imports (see rwxrob/{help,conf,vars})
		help.Cmd, conf.Cmd, vars.Cmd,

		// local commands (in this module)
		LoopCmd, ClearCmd, PywalCmd, dependencies.TestDepsCmd},

	Description: `
		Lazywal: a terminal client to help setup video-loops/gifs as background.

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
