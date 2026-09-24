package loop

import (
	"fmt"
	"log"

	"github.com/BuddhiLW/bonzai"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

// setArgs are the parsed arguments of `lazywal set`.
type setArgs struct {
	path   string
	colors bool
	// display is accepted for compatibility with older scripts only:
	// monitors are always detected.
	display string
}

// parseSetArgs parses `<path> [display <WxH>] [colors|pywal|update-pywal]`.
func parseSetArgs(args []string) (setArgs, error) {
	if len(args) == 0 {
		return setArgs{}, fmt.Errorf("missing video path")
	}
	a := setArgs{path: args[0]}
	rest := args[1:]
	if n := len(rest); n > 0 && Matches(PywalCmd, rest[n-1]) {
		a.colors = true
		rest = rest[:n-1]
	}
	switch {
	case len(rest) == 0:
	case len(rest) == 2 && rest[0] == "display":
		a.display = rest[1]
	default:
		return setArgs{}, fmt.Errorf("unexpected arguments: %q", rest)
	}
	return a, nil
}

var LoopCmd = &bonzai.Cmd{
	Name:    `set`,
	Alias:   `set-path|path`,
	Usage:   `lazywal set <path> [colors]`,
	Short:   `set video wallpaper from given path`,
	MinArgs: 1,
	Cmds:    []*bonzai.Cmd{HelpCmd},

	// MCP metadata for tool generation
	Mcp: &bonzai.McpMeta{
		Desc: "Set a video or animated GIF as wallpaper on all monitors",
		Params: []bonzai.McpParam{
			{Name: "path", Desc: "Absolute path to video or GIF file", Type: "string", Required: true},
		},
	},

	Do: func(x *bonzai.Cmd, args ...string) error {
		a, err := parseSetArgs(args)
		if err != nil {
			return fmt.Errorf("%w (usage: %s)", err, x.Usage)
		}
		if a.display != "" {
			log.Printf("Ignoring display %s: monitors are detected automatically", a.display)
		}
		video, err := wallpaper.NewVideo(a.path)
		if err != nil {
			return err
		}
		log.Print("File chosen: ", video)

		app, err := newApp()
		if err != nil {
			return err
		}
		if _, err := app.Service.Set(video); err != nil {
			return err
		}
		if a.colors {
			return app.Colors.Apply(video)
		}
		return nil
	},
}

var PywalCmd = &bonzai.Cmd{
	Name:    `pywal`,
	Alias:   `update-pywal|colors`,
	Usage:   `lazywal pywal`,
	Short:   `apply pywal colors from random video frame`,
	NumArgs: 0,
	Cmds:    []*bonzai.Cmd{HelpCmd},

	// MCP metadata for tool generation
	Mcp: &bonzai.McpMeta{
		Desc: "Extract a random frame from the current wallpaper and apply pywal color scheme",
	},

	Do: func(_ *bonzai.Cmd, _ ...string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		video, err := app.Service.Current()
		if err != nil {
			return err
		}
		return app.Colors.Apply(video)
	},
}

var ClearCmd = &bonzai.Cmd{
	Name:    `clear`,
	Alias:   `kill`,
	Usage:   `lazywal clear`,
	Short:   `stop the video wallpaper and restore the desktop`,
	MinArgs: 0,
	Cmds:    []*bonzai.Cmd{HelpCmd},

	// MCP metadata for tool generation
	Mcp: &bonzai.McpMeta{
		Desc: "Clear the video wallpaper using the backend that set it",
	},

	Do: func(_ *bonzai.Cmd, _ ...string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		b, err := app.Service.Clear()
		switch {
		case err != nil:
			return err
		case b == nil:
			fmt.Println("Nothing to clear")
		default:
			fmt.Printf("Cleared video wallpaper (backend: %s)\n", b.Name())
		}
		return nil
	},
}

// Matches checks if arg matches the command's name or any of its aliases
func Matches(cmd *bonzai.Cmd, arg string) bool {
	if arg == cmd.Name {
		return true
	}
	for _, alias := range cmd.Aliases() {
		if arg == alias {
			return true
		}
	}
	return false
}
