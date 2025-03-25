package loop

import (
	"os/exec"
	"testing"

	Z "github.com/rwxrob/bonzai/z"
)

func testSetupWallpaper(t *testing.T) *Wallpaper {
	return NewWallPaper(&Config{Path: "test.mp4"})
}

func testMockCommand(output string) *exec.Cmd {
	return exec.Command("echo", output)
}

func testContains(slice []int, item int) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

func cleanupVars() {
	Z.Vars.Del(VarPIDs)
	monitors, _ := GetMonitors()
	for _, monitor := range monitors {
		Z.Vars.Del(VarMonitorPIDs + "_" + monitor.Name)
	}
	// Kill any stray processes
	exec.Command("pkill", "-f", "xwinwrap").Run()
	exec.Command("pkill", "-f", "mpv").Run()
}
