package loop

import (
	"fmt"
	"log"
	"os/exec"
	"syscall"

	dependencies "github.com/BuddhiLW/lazywal/internal/check"
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
	Config   *Config
	Running  map[string]*exec.Cmd // Track running processes per monitor
	Monitors []Monitor
}

func NewWallPaper(setup *Config) *Wallpaper {
	return &Wallpaper{
		Config:  setup,
		Running: make(map[string]*exec.Cmd),
	}
}

var (
	Wall           *Wallpaper = NewWallPaper(NewConfig())
	defaultDisplay string     = GetDefaultDisplay()
	mpvArgs        string     = "-wid WID --loop --no-audio --no-resume-playback --panscan=1.0"
)

func (w *Wallpaper) Set() error {
	monitors, err := GetMonitors()
	if err != nil {
		return fmt.Errorf("failed to get monitor info: %w", err)
	}

	// Kill existing processes
	w.killExisting()

	// Start new processes for each monitor
	for _, monitor := range monitors {
		if err := w.startOnMonitor(monitor); err != nil {
			log.Printf("Failed to start on monitor %s: %v", monitor.Name, err)
			continue
		}
	}

	return nil
}

func (w *Wallpaper) startOnMonitor(monitor Monitor) error {
	geometry := fmt.Sprintf("%dx%d+%d+%d",
		int(monitor.Dimensions.Width),
		int(monitor.Dimensions.Height),
		int(monitor.Position.X),
		int(monitor.Position.Y))

	xwinwrapArgs := fmt.Sprintf("-g %s -ni -b -st -un -o 1.0 -ov -debug", geometry)
	commandString := fmt.Sprintf("xwinwrap %s -- mpv %s '%s'", xwinwrapArgs, mpvArgs, w.Config.Path)

	log.Printf("Running command: %s", commandString)
	cmd := exec.Command("bash", "-c", commandString)

	// Set up process to run in its own process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Create new process group
		Pgid:    0,    // New process group
	}

	// Redirect stdout/stderr to /dev/null to prevent pipe issues
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Don't wait for the process, let it run in background
	w.Running[monitor.Name] = cmd
	log.Printf("Started wallpaper on monitor %s (PID: %d)", monitor.Name, cmd.Process.Pid)

	return nil
}

func (w *Wallpaper) killExisting() {
	for monitor, cmd := range w.Running {
		if cmd != nil && cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				log.Printf("Failed to kill process on monitor %s: %v", monitor, err)
			}
		}
	}
	w.Running = make(map[string]*exec.Cmd)
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
			err := help.Cmd.Call(caller, "help")
			if err != nil {
				return err
			}
			return nil
		}

		path := args[0]
		if !validPath(path) {
			log.Fatal("Invalid Path")
		}

		// Set the path before checking dependencies
		log.Print("File chosen: ", path)
		Wall.Config.Path = path

		err := dependencies.TestDepsCmd.Call(caller, args[0])
		if err != nil {
			return err
		}

		if len(args) < 2 {
			err := SetDisplayCmd.Call(caller, defaultDisplay)
			if err != nil {
				return err
			}
			return nil
		}

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
		return Wall.Set()
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
		available, err := dependencies.IsWalAvailable()
		if available {
			fmt.Println("wal is available in your system.")
		} else {
			return err
		}

		Wall.Pywal()
		return nil
	},
}
