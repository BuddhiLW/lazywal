package loop

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rwxrob/bonzai"
)

var execCommand = exec.Command

type Size struct {
	Width  float32
	Height float32
}

func validPath(path string) bool {
	_, errStat := os.Stat(path)
	return errStat == nil
}

func validDimension(dimension string) bool {
	size, err := parseSize(dimension)

	if err != nil {
		return false
	}

	return size != nil

	//-------------------------------------
	//
	// TODO: some kind of test for dimensions max-dimensions?
	// PROBLEM: More than one screen use
	//
	//-------------------------------------
	//-------------------------------------
	//
	// screens, err := xrandr.GetScreens()
	// if err != nil {
	// 	return false
	// }

	// for screen := range screens {
	// 	currentScreenWidth := screen.CurrentResolution.Width
	// 	currentScreenHeight := screen.CurrentResolution.Height
	// }

	// // return size != nil && screens != nil
}

func parseSize(s string) (*Size, error) {
	if !strings.Contains(s, "x") {
		return nil, fmt.Errorf("invalid size format; expected format WxH but got %s", s)
	}

	res := strings.Split(s, "x")
	width, err := strconv.Atoi(strings.TrimSpace(res[0]))
	if err != nil {
		return nil, fmt.Errorf("could not parse mode width size (%s): %s", s, err)
	}

	height, err := strconv.Atoi(strings.TrimSpace(res[1]))
	if err != nil {
		return nil, fmt.Errorf("could not parse mode height size (%s): %s", s, err)
	}

	return &Size{
		Width:  float32(width),
		Height: float32(height),
	}, nil
}

func GetDefaultDisplay() string {
	cmd := execCommand("bash", "-c", "xdpyinfo | grep dimensions | sed -r 's/^[^0-9]*([0-9]+x[0-9]+).*$/\\1/'")
	out, err := cmd.Output()
	if err != nil {
		log.Println(err)
		panic(err)
	}
	return strings.TrimSpace(string(out))
}

// Matches checks if arg matches the command's name or any of its aliases
func Matches(cmd *bonzai.Cmd, arg string) bool {
	if arg == cmd.Name {
		return true
	}
	// In new bonzai, Alias is pipe-delimited string, use Aliases() method
	for _, alias := range cmd.Aliases() {
		if arg == alias {
			return true
		}
	}
	return false
}

func GetMonitors() ([]Monitor, error) {
	cmd := execCommand("xrandr", "--current")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get monitor info: %w", err)
	}

	var monitors []Monitor
	lines := strings.Split(string(out), "\n")

	for _, line := range lines {
		if strings.Contains(line, " connected ") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}

			name := fields[0]
			var geometry string
			isPrimary := strings.Contains(line, "primary")

			// Find the active resolution and position
			for _, field := range fields {
				if strings.Contains(field, "x") && strings.Contains(field, "+") {
					geometry = field
					break
				}
			}

			if geometry == "" {
				continue
			}

			// Parse geometry string (e.g., "1920x1080+1920+0")
			parts := strings.Split(geometry, "+")
			if len(parts) != 3 {
				continue
			}

			size, err := parseSize(parts[0])
			if err != nil {
				continue
			}

			x, err := strconv.ParseFloat(parts[1], 32)
			if err != nil {
				continue
			}

			y, err := strconv.ParseFloat(parts[2], 32)
			if err != nil {
				continue
			}

			monitors = append(monitors, Monitor{
				Name:       name,
				Dimensions: *size,
				Position: Position{
					X: float32(x),
					Y: float32(y),
				},
				Primary: isPrimary,
			})
		}
	}

	if len(monitors) == 0 {
		return nil, fmt.Errorf("no active monitors found")
	}

	return monitors, nil
}
