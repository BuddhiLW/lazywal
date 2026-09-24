package loop

import (
	"fmt"

	"github.com/BuddhiLW/bonzai"
	"github.com/BuddhiLW/lazywal/internal/backend/custom"
)

var BackendsCmd = &bonzai.Cmd{
	Name:    `backends`,
	Alias:   `backend`,
	Usage:   `lazywal backends`,
	Short:   `list backends and the one this session uses`,
	NumArgs: 0,
	Cmds:    []*bonzai.Cmd{HelpCmd},

	Mcp: &bonzai.McpMeta{
		Desc: "List wallpaper backends, whether each is ready, and which one the current session uses",
	},

	Do: func(_ *bonzai.Cmd, _ ...string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		svc := app.Service
		fmt.Println(svc.Session)

		selected, _ := svc.Backend()
		for _, b := range svc.Registry.All() {
			mark := " "
			if selected != nil && b.Name() == selected.Name() {
				mark = "*"
			}
			status := "ready"
			if err := b.Check(); err != nil {
				status = err.Error()
			}
			fmt.Printf("%s %-11s %s\n  %11s  %s\n", mark, b.Name(), b.Describe(), "", status)
		}
		fmt.Printf("\n* = selected; override with %s=<name>\n", EnvBackend)
		fmt.Printf("Add your own backends in %s\n", custom.DefaultPath())
		return nil
	},
}

// DepsCmd checks the dependencies of the backend this session would use.
var DepsCmd = &bonzai.Cmd{
	Name:    `test`,
	Alias:   `dependencies`,
	Usage:   `lazywal test`,
	Short:   `test to see if all dependencies are available`,
	NumArgs: 0,

	Do: func(_ *bonzai.Cmd, _ ...string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		b, selErr := app.Service.Backend()
		if b == nil {
			return selErr
		}
		if optional := app.Colors.Check(); optional != nil {
			fmt.Printf("Optional, for colors: %s\n", optional)
		}
		if err := b.Check(); err != nil {
			fmt.Println("Lazywal won't run.")
			return fmt.Errorf("%sbackend %s: %s%s%s", yellow, b.Name(), red, err, reset)
		}
		fmt.Printf("All dependencies installed (backend: %s)\n", b.Name())
		return nil
	},
}

const (
	reset  = "\033[0m"
	red    = "\033[31m"
	yellow = "\033[33m"
)
