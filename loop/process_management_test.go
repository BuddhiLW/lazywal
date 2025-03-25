package loop

import (
	"os/exec"
	"strings"
	"testing"

	//"time"

	Z "github.com/rwxrob/bonzai/z"
)

func TestProcessManagementSuite(t *testing.T) {
	// Setup for all tests
	if err := Z.Vars.Init(); err != nil {
		t.Fatalf("Failed to initialize Z.Vars: %v", err)
	}
	originalExec := execCommand
	defer func() {
		execCommand = originalExec
		cleanupVars()
		// Double-check cleanup
		exec.Command("pkill", "-f", "xwinwrap").Run()
		exec.Command("pkill", "-f", "mpv").Run()
	}()

	t.Run("Process replacement", func(t *testing.T) {
		w := testSetupWallpaper(t)
		monitor := Monitor{Name: "test-monitor", Dimensions: Size{Width: 1920, Height: 1080}}

		// Mock command that doesn't create real processes
		execCommand = func(name string, args ...string) *exec.Cmd {
			return testMockCommand("mock output")
		}

		if err := w.startOnMonitor(monitor); err != nil {
			t.Fatalf("Failed to start process: %v", err)
		}

		// Basic verification that process is tracked
		if cmd := w.Running[monitor.Name]; cmd == nil {
			t.Error("Process not tracked in Running map")
		}
	})
}

// TODO: Fix PID tracking tests
// TODO: Fix process cleanup tests
// TODO: Add proper process lifecycle tests

// Helper functions

func setupTestWallpaper(t *testing.T) *Wallpaper {
	return NewWallPaper(&Config{Path: "test.mp4"})
}

func mockExecCommand(childPIDs []string) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		if strings.Contains(strings.Join(args, " "), "pgrep") {
			return exec.Command("echo", strings.Join(childPIDs, "\n"))
		}
		if strings.Contains(strings.Join(args, " "), "pkill") {
			return exec.Command("true")
		}
		return exec.Command("true")
	}
}

func contains(slice []int, item int) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}
