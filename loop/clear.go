package loop

import (
	"fmt"
	"os/exec"

	"github.com/BuddhiLW/bonzai"
)

var ClearCmd = &bonzai.Cmd{
	Name:    `clear`,
	Alias:   `kill`,
	Usage:   `lazywal clear`,
	Short:   `kill all xwinwrap processes that may be hanging`,
	MinArgs: 0,
	Cmds:    []*bonzai.Cmd{HelpCmd},

	// MCP metadata for tool generation
	Mcp: &bonzai.McpMeta{
		Desc: "Clear the wallpaper by killing all xwinwrap processes",
	},

	Do: func(x *bonzai.Cmd, _ ...string) error {
		fmt.Println("Killing all xwinwrap processes")
		cmd := exec.Command("pkill", "xwinwrap")
		return cmd.Run()
	},
}
