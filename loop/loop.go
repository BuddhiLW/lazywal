package loop

import (
	"fmt"
	"log"
	"os/exec"
	"syscall"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"
)

type Config struct {
	Path       string
	Dimensions *Size
	LastUsed   string // still not used
}

func NewConfig() *Config {
	size, _ := parseSize(defaultDisplay)
	return &Config{Dimensions: size}
}

type Wallpaper struct {
	Config *Config
}

func NewWallPaper(setup *Config) *Wallpaper {
	// Path:       setup["Path"],
	// Dimensions: setup["Dimensions"],
	// LastUsed:   setup["Last_used"],
	return &Wallpaper{
		Config: setup,
	}
}

var (
	Wall           *Wallpaper = NewWallPaper(NewConfig())
	defaultDisplay string     = GetDefaultDisplay()
	xwinwrapArgs   string     = fmt.Sprintf("-g %v -ni -b -st -un -o 1.0 -ov -debug", defaultDisplay)
	mpvArgs        string     = "-wid WID --loop --no-audio --no-resume-playback --panscan=1.0"
)

func (w *Wallpaper) Set() {
	commandString := fmt.Sprintf("xwinwrap %s -- mpv %s %s", xwinwrapArgs, mpvArgs, w.Config.Path)
	cmd := exec.Command("bash", "-c", commandString)

	// Detach process from parent
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	err := cmd.Start()
	if err != nil {
		log.Println("Error starting command:", err)
		return
	}
	if Z.Vars.Get("PID") != "" {
		exec.Command("bash", "-c", fmt.Sprintf("kill -9 %s", Z.Vars.Get("PID"))).Run()
	}

	Z.Vars.Set("PID", fmt.Sprintf("%v", cmd.Process.Pid))
	log.Println("Running PID: ", Z.Vars.Get("PID"))
}

var LoopCmd = &Z.Cmd{
	Name:     `set`,
	Aliases:  []string{`set-path`, `path`},
	Usage:    `<path> display <dimension>`,
	Summary:  `Renders the file in <path> as wallpaper with specified display dimension.`,
	MinArgs:  0,
	Commands: []*Z.Cmd{help.Cmd, SetDisplayCmd},
	Call: func(caller *Z.Cmd, args ...string) error {
		if len(args) == 0 {
			help.Cmd.Call(caller, "help")
			return nil
		}

		if len(args) < 2 {
			err := SetDisplayCmd.Call(caller, defaultDisplay)
			if err != nil {
				return err
			}
			return nil
		}

		path := args[0]
		if !validPath(path) {
			log.Fatal("Invalid Path")
		}

		log.Print("File chosen: ", path)
		Wall.Config.Path = args[0]

		if len(args) > 2 && args[1] == "display" {
			err := SetDisplayCmd.Call(caller, args[2:]...)
			if err != nil {
				return err
			}
		}

		// if last `args` is any of PywalCmd.Aliases or PywalCmd.Name
		// Then, update pywal schema
		if len(args) > 0 && Matches(PywalCmd, args[len(args)-1]) {
			err := PywalCmd.Call(caller)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

var SetDisplayCmd = &Z.Cmd{
	Name:     `display`,
	Aliases:  []string{`setdisplay`, `set`},
	Usage:    `<path>`,
	Summary:  `Set wallpaper to dimensions/position of screen <path>.`,
	NumArgs:  1,
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(_ *Z.Cmd, args ...string) error {
		err := SetDisplay(args[0])
		if err != nil {
			return err
		}
		Wall.Set()
		return nil
	},
}

var PywalCmd = &Z.Cmd{
	Name:     `pywal`,
	Aliases:  []string{"update-pywal", "colors"},
	Usage:    `lazywal set <path> colors`,
	Summary:  `Update pywal scheme to use a random frame from the loop.`,
	NumArgs:  0,
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(_ *Z.Cmd, args ...string) error {
		Wall.Pywal()
		return nil
	},
}
