package loop

import (
	"fmt"
	"os/exec"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"
)

var ClearCmd = &Z.Cmd{
	Name:     `clear`,
	Aliases:  []string{`kill`},
	Usage:    `lazywal clear`,
	Summary:  `Kill all process related with 'xwinwrap' that may be hanging.`,
	MinArgs:  0,
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(caller *Z.Cmd, _ ...string) error {
		fmt.Println("Killing all xwinwrap processes")
		cmd := exec.Command("pkill", "xwinwrap")
		return cmd.Run()
	},
}
