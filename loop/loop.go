package loop

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	dependencies "github.com/BuddhiLW/lazywal/internal/check"
	"github.com/rwxrob/bonzai"
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

const (
	VarPrefix      = "lazywal_"
	VarPath        = VarPrefix + "path"
	VarPIDs        = VarPrefix + "pids"
	VarMonitorPIDs = VarPrefix + "monitor_pids"
)

type Wallpaper struct {
	Config   *Config
	Running  map[string]*exec.Cmd
	Monitors []Monitor
	// Simple in-memory storage for PIDs (replacing vars.Data)
	pids        map[string]string
}

func NewWallPaper(setup *Config) *Wallpaper {
	return &Wallpaper{
		Config:  setup,
		Running: make(map[string]*exec.Cmd),
		pids:    make(map[string]string),
	}
}

var (
	Wall           *Wallpaper = NewWallPaper(NewConfig())
	defaultDisplay string     = GetDefaultDisplay()
	mpvArgs        string     = "-wid WID --loop --no-audio --no-resume-playback --panscan=1.0"
)

// Simple in-memory variable storage methods
func (w *Wallpaper) getVar(key string) string {
	return w.pids[key]
}

func (w *Wallpaper) setVar(key, value string) {
	w.pids[key] = value
}

func (w *Wallpaper) delVar(key string) {
	delete(w.pids, key)
}

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

func (w *Wallpaper) getMonitorPIDs(monitor string) []int {
	pidsStr := w.getVar(VarMonitorPIDs + "_" + monitor)
	if pidsStr == "" {
		return nil
	}
	var pids []int
	for _, pidStr := range strings.Split(pidsStr, ",") {
		if pid, err := strconv.Atoi(pidStr); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

func (w *Wallpaper) setMonitorPIDs(monitor string, pids []int) {
	pidStrs := make([]string, len(pids))
	for i, pid := range pids {
		pidStrs[i] = strconv.Itoa(pid)
	}
	w.setVar(VarMonitorPIDs+"_"+monitor, strings.Join(pidStrs, ","))
}

func (w *Wallpaper) getAllPIDs() []int {
	pidsStr := w.getVar(VarPIDs)
	if pidsStr == "" {
		return nil
	}
	var pids []int
	for _, pidStr := range strings.Split(pidsStr, ",") {
		if pid, err := strconv.Atoi(pidStr); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

func (w *Wallpaper) setAllPIDs(pids []int) {
	pidStrs := make([]string, len(pids))
	for i, pid := range pids {
		pidStrs[i] = strconv.Itoa(pid)
	}
	w.setVar(VarPIDs, strings.Join(pidStrs, ","))
}

func (w *Wallpaper) startOnMonitor(monitor Monitor) error {
	// First kill any existing process for this monitor
	if oldPIDs := w.getMonitorPIDs(monitor.Name); len(oldPIDs) > 0 {
		// Kill all processes associated with this monitor
		for _, pid := range oldPIDs {
			if runtime.GOOS != "windows" {
				if pgid, err := syscall.Getpgid(pid); err == nil {
					syscall.Kill(-pgid, syscall.SIGKILL)
				}
			}
			syscall.Kill(pid, syscall.SIGKILL)
		}

		// Remove these PIDs from global tracking
		allPIDs := w.getAllPIDs()
		oldPIDMap := make(map[int]bool)
		for _, pid := range oldPIDs {
			oldPIDMap[pid] = true
		}
		newPIDs := make([]int, 0)
		for _, pid := range allPIDs {
			if !oldPIDMap[pid] {
				newPIDs = append(newPIDs, pid)
			}
		}
		w.setAllPIDs(newPIDs)
		w.delVar(VarMonitorPIDs + "_" + monitor.Name)
	}

	// Delete from Running map
	delete(w.Running, monitor.Name)

	// Start new process
	geometry := fmt.Sprintf("%dx%d+%d+%d",
		int(monitor.Dimensions.Width),
		int(monitor.Dimensions.Height),
		int(monitor.Position.X),
		int(monitor.Position.Y))

	xwinwrapArgs := fmt.Sprintf("-g %s -ni -b -st -un -o 1.0 -ov -debug", geometry)
	commandString := fmt.Sprintf("xwinwrap %s -- mpv %s '%s'", xwinwrapArgs, mpvArgs, w.Config.Path)

	log.Printf("Running command: %s", commandString)
	cmd := exec.Command("bash", "-c", commandString)

	// Set up process group
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

	// Track the main process PID
	mainPID := cmd.Process.Pid

	// Track PIDs for this monitor
	monitorPIDs := []int{mainPID}
	allPIDs := w.getAllPIDs()
	allPIDs = append(allPIDs, mainPID)

	// Get child PIDs
	childCmd := exec.Command("bash", "-c", fmt.Sprintf(`pgrep -P %d; pgrep -f "mpv.*%d"`, mainPID, mainPID))
	output, err := childCmd.Output()
	if err == nil {
		seen := make(map[int]bool)
		seen[mainPID] = true

		for _, pidStr := range strings.Fields(string(output)) {
			if pid, err := strconv.Atoi(pidStr); err == nil && !seen[pid] {
				monitorPIDs = append(monitorPIDs, pid)
				allPIDs = append(allPIDs, pid)
				seen[pid] = true
			}
		}
	}

	// Store PIDs
	w.setMonitorPIDs(monitor.Name, monitorPIDs)
	w.setAllPIDs(allPIDs)
	w.Running[monitor.Name] = cmd

	log.Printf("Started wallpaper on monitor %s (PIDs: %v)", monitor.Name, monitorPIDs)

	return nil
}

func (w *Wallpaper) killExisting() {
	// Kill all tracked processes
	allPIDs := w.getAllPIDs()
	for _, pid := range allPIDs {
		if runtime.GOOS != "windows" {
			if pgid, err := syscall.Getpgid(pid); err == nil {
				syscall.Kill(-pgid, syscall.SIGKILL)
			}
		}
		syscall.Kill(pid, syscall.SIGKILL)
	}

	// Clear all tracking data
	w.Running = make(map[string]*exec.Cmd)
	w.delVar(VarPIDs)

	// Clear monitor PIDs
	monitors, _ := GetMonitors()
	for _, monitor := range monitors {
		w.delVar(VarMonitorPIDs + "_" + monitor.Name)
	}
}

var LoopCmd = &bonzai.Cmd{
	Name:    `set`,
	Alias:   `set-path|path`,
	Usage:   `lazywal set <path> [display <dimension>]`,
	Short:   `set video wallpaper from given path`,
	MinArgs: 1,
	Cmds:    []*bonzai.Cmd{HelpCmd, SetDisplayCmd},

	// MCP metadata for tool generation
	Mcp: &bonzai.McpMeta{
		Desc: "Set a video or animated GIF as wallpaper on all monitors",
		Params: []bonzai.McpParam{
			{Name: "path", Desc: "Absolute path to video or GIF file", Type: "string", Required: true},
		},
	},

	Do: func(x *bonzai.Cmd, args ...string) error {
		path := args[0]
		if !validPath(path) {
			return fmt.Errorf("invalid path: %s", path)
		}

		// Set the path before checking dependencies
		log.Print("File chosen: ", path)
		Wall.Config.Path = path

		err := dependencies.TestDepsCmd.Do(x, args[0])
		if err != nil {
			return err
		}

		if len(args) < 2 {
			err := SetDisplayCmd.Do(x, defaultDisplay)
			if err != nil {
				return err
			}
			return nil
		}

		if len(args) > 2 && args[1] == "display" {
			err := SetDisplayCmd.Do(x, args[2:]...)
			if err != nil {
				return err
			}
		}

		// if last `args` is any of PywalCmd.Aliases or PywalCmd.Name
		if len(args) > 0 && Matches(PywalCmd, args[len(args)-1]) {
			err := PywalCmd.Do(x)
			if err != nil {
				return err
			}
		}

		return nil
	},
}

var PywalCmd = &bonzai.Cmd{
	Name:    `pywal`,
	Alias:   `update-pywal|colors`,
	Usage:   `lazywal set <path> colors`,
	Short:   `apply pywal colors from random video frame`,
	NumArgs: 0,
	Cmds:    []*bonzai.Cmd{HelpCmd},

	// MCP metadata for tool generation
	Mcp: &bonzai.McpMeta{
		Desc: "Extract a random frame from the current wallpaper and apply pywal color scheme",
	},

	Do: func(_ *bonzai.Cmd, args ...string) error {
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

var SetDisplayCmd = &bonzai.Cmd{
	Name:    `display`,
	Alias:   `setdisplay|set`,
	Usage:   `<path>`,
	Short:   `set wallpaper to screen dimensions`,
	NumArgs: 1,
	Cmds:    []*bonzai.Cmd{HelpCmd},

	Do: func(_ *bonzai.Cmd, args ...string) error {
		err := SetDisplay(args[0])
		if err != nil {
			return err
		}
		return Wall.Set()
	},
}
