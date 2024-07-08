package loop

import (
	"fmt"
	"testing"

	"github.com/vcraescu/go-xrandr"
)

func TestXrandrCommands(t *testing.T) {
	screens, _ := xrandr.GetScreens()
	fmt.Println(screens[0].CurrentResolution)
}

func TestGetDefaultDisplay(t *testing.T) {
	fmt.Println(GetDefaultDisplay())
}
