package dependencies

import (
	"fmt"
	"os/exec"

	"github.com/BuddhiLW/lazywal/config"
	"github.com/rwxrob/bonzai"
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
		return remaining, false
	}

	return "", true
}

func IsWalAvailable() (bool, error) {
	_, err := exec.LookPath("wal")
	if err != nil {
		return false, fmt.Errorf("wal is not available in your system: %w", err)
	}
	return true, nil
}

var TestDepsCmd = &bonzai.Cmd{
	Name:    `test`,
	Alias:   `dependencies`,
	Usage:   `lazywal test`,
	Short:   `test to see if all dependencies are available`,
	NumArgs: 0,

	Do: func(_ *bonzai.Cmd, args ...string) error {
		remainingDeps, ready := CheckReady()
		if ready {
			fmt.Println("All dependencies installed")
			return nil
		}
		fmt.Println("Lazywal won't run.")
		return fmt.Errorf("%sDependencies not found: %s%s%s", Yellow, Red, remainingDeps, Reset)
	},
}
