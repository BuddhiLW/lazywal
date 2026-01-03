package loop

import (
	"os/exec"
	"testing"
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
	// Clear persistent storage
	Wall.delVar(VarPIDs)
	Wall.Running = make(map[string]*exec.Cmd)
	// Kill any stray processes
	exec.Command("pkill", "-f", "xwinwrap").Run()
	exec.Command("pkill", "-f", "mpv").Run()
}
