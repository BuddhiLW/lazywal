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
	Summary:   `Lazywal: a terminal client to help setup video-loops/gifs as background.`,
	Version:   `v1.0.2`,
	Copyright: `Copyright 2021-2024 Zayac-The-Engineer, 2024 Pedro G. Branquinho`,
	License:   `MIT License`,
	Site:      `buddhilw.com`,
	Source:    `git@github.com/BuddhiLW/lazywal-cli.git`,
	Issues:    `github.com/BuddhiLW/lazywal-cli/issues`,

	Commands: []*Z.Cmd{
		// standard external branch imports (see rwxrob/{help,conf,vars})
		help.Cmd, conf.Cmd, vars.Cmd,

		// local commands (in this module)
		LoopCmd,
	},

	// TODO: increment Description
	Description: `
		Lazywal: a terminal client to help setup video-loops/gifs as background.
		`,

	// -------------------------
	// Description template:
	// -------------------------
	// Quake Report is a Bonzai composite command tree, for generating Reports for Cloud Walk technical test.

	// It is a simple CLI application that generates a report for the given earthquake data.

	// You can use the following commands to generate the report:

	// * qrep **command** help	(documentation for the **command**)
	// * qrep data
	// * qrep data **n**		(where **n** is the match you want json-formatted data about)
	// * qrep data rank **n**	(where **n** is the given match-ranking)
	// * qrep data stats **n** 	(where **n** is the given match-stats -- kills by type)

	// See the README.md for more information and examples, or use *_command-tree_ help* to see another man-page about the specific command-tree.
	// -------------------------
}
