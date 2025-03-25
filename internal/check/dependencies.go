package dependencies

import (
	"fmt"
	"os/exec"

	"github.com/BuddhiLW/lazywal/config"
	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Yellow = "\033[33m"
)

func CheckReady() (string, bool) {
	binaries := config.DepsList

	var remaining string
	for _, binary := range binaries {
		_, err := exec.LookPath(binary)
		if err != nil {
			if remaining == "" {
				remaining = binary
			} else {
				remaining += ", " + binary
			}
		}
	}

	if remaining != "" {
		if err := Z.Vars.Set("RemainingDeps", remaining); err != nil {
			return remaining, false
		}
		return remaining, false
	}

	if err := Z.Vars.Set("RemainingDeps", "none"); err != nil {
		return "", false
	}
	return "", true
}

func IsWalAvailable() (bool, error) {
	// Use exec.LookPath to check if "wal" exists in the system's PATH
	_, err := exec.LookPath("wal")
	if err != nil {
		return false, fmt.Errorf("wal is not available in your system: %w", err)
	}
	return true, nil
}

var TestDepsCmd = &Z.Cmd{
	Name:     `test`,
	Aliases:  []string{"dependencies"},
	Usage:    `lazywal test`,
	Summary:  `Test to see if all dependencies are available in your system.`,
	NumArgs:  0,
	Commands: []*Z.Cmd{help.Cmd},
	Call: func(_ *Z.Cmd, args ...string) error {
		remainingDeps, ready := CheckReady()
		if ready {
			fmt.Println("All dependencies installed")
			return nil
		}
		fmt.Println("Lazywal won't run.")
		return fmt.Errorf("%sDependencies not found in your system: %s%s%s", Yellow, Red, remainingDeps, Reset)
	},
}
