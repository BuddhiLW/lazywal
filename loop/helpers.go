package loop

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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
	cmd := exec.Command("bash", "-c", "xdpyinfo | grep dimensions | sed -r 's/^[^0-9]*([0-9]+x[0-9]+).*$/\\1/'")
	out, err := cmd.Output()
	str := strings.Trim(string(out), "\n")
	if err != nil {
		log.Println(err)
		panic(err)
	}
	// fmt.Println(string(out))

	return str
}
