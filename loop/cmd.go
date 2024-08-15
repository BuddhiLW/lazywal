// Copyright 2024 qrep Pedro G. Branquinho
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/conf"
	"github.com/rwxrob/help"
	"github.com/rwxrob/vars"
)

func init() {
	Z.Conf.SoftInit()
	Z.Vars.SoftInit()
}

var Cmd = &Z.Cmd{
	Name:      `lazywal`,
	Summary:   `Lazywal: a terminal client to facilitate setting up video-loops/gifs as background.`,
	Version:   `v0.5.3`,
	Copyright: `Copyright 2021-2024 Zayac-The-Engineer, 2024 Pedro G. Branquinho (Go version)`,
	License:   `MIT License`,
	Site:      `buddhilw.com`,
	Source:    `git@github.com/BuddhiLW/lazywal.git`,
	Issues:    `github.com/BuddhiLW/lazywal/issues`,

	Commands: []*Z.Cmd{
		// standard external branch imports (see rwxrob/{help,conf,vars})
		help.Cmd, conf.Cmd, vars.Cmd,

		// local commands (in this module)
		LoopCmd, ClearCmd, PywalCmd,
	},

	// TODO: increment Description
	Description: `
		Lazywal: a terminal client to help setup video-loops/gifs as background.

		You can use the following commands:

		* lazywal set **path**  			   (Tries to get first-display screen-size automatically)
		* lazywal set **path** display **WxH** (Width x Height - e.g. 1440x1080, 2560x1080 etc.)
		* lazywal kill 						   (Kills all _xwinwrap_ processes running.)

		Note: **path** should be the path to the video-loop file.

		See the README.md for more information and examples, or use *_command-tree_ help* to see another man-page about the specific command-tree.
		`,
}
