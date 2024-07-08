package loop

import (
	"fmt"
	"log"
	"os/exec"

	Z "github.com/rwxrob/bonzai/z"
	"github.com/rwxrob/help"
)

type Config struct {
	Path       string
	Dimensions *Size
	LastUsed   string
}

func NewConfig() *Config {
	size, _ := parseSize(defaultDisplay)
	return &Config{Dimensions: size}
}

type Wallpaper struct {
	Config *Config
}

func NewWallPaper(setup *Config) *Wallpaper {
	// Path:       setup["Path"],
	// Dimensions: setup["Dimensions"],
	// LastUsed:   setup["Last_used"],
	return &Wallpaper{
		Config: setup,
	}
}

var (
	Wall           *Wallpaper = NewWallPaper(NewConfig())
	defaultDisplay string     = GetDefaultDisplay()
	xwinwrapArgs   string     = fmt.Sprintf("-g %v -ni -b -st -un -o 1.0 -ov -debug", defaultDisplay)
	mpvArgs        string     = "-wid WID --loop --no-audio --no-resume-playback --panscan=1.0"
)

func (w *Wallpaper) Set() {
	log.Println("Setting wallpaper: ", w.Config, &w.Config.Dimensions)
	log.Println("defaultDisplay: ", defaultDisplay)
	log.Println("xwinwrapArgs: ", xwinwrapArgs)
	log.Println("Complete command: ", "bash", "-c", "xwinwrap", xwinwrapArgs, "--", "mpv", mpvArgs, w.Config.Path)
	commandString := fmt.Sprintf("xwinwrap %s -- mpv %s %s", xwinwrapArgs, mpvArgs, w.Config.Path)

	// cmd := exec.Command("bash", "-c", "xdpyinfo | grep dimensions | sed -r 's/^[^0-9]*([0-9]+x[0-9]+).*$/\\1/'")
	// c1 := exec.Command("bash", "-c", "xwinwrap", xwinwrapArgs, "--", "mpv", mpvArgs, w.Config.Path)

	// pkill xwinwrap
	// sleep 0.05
	// pkill xwinwrap && sleep 0.05
	c1 := exec.Command("bash", "-c", fmt.Sprintf("$(%s)", commandString))
	err := c1.Run()

	log.Println(err)
}

var LoopCmd = &Z.Cmd{
	Name:     `loop`,
	Aliases:  []string{`vl`, `l`, `videoloop`},
	Usage:    `<path>`,
	Summary:  `Renders the file in *path* (a video-loop), as wallpaper.`,
	MinArgs:  1,
	Commands: []*Z.Cmd{help.Cmd, SetDisplayCmd}, // RankCmd, StatisticsCmd
	Call: func(caller *Z.Cmd, args ...string) error {
		if len(args) == 0 {
			log.Print("Give a <file-path>, in order to set it as wallpaper")
			return nil
		} else {
			log.Print("File chosen: ", args[0])
			// Test if args[0] is a valid path
			if validPath(args[0]) {
				Wall.Config.Path = args[0]
				// caller.Call(SetDisplayCmd)
				// .Params(args[2])
				if args[1] != "" && args[2] != "" {
					err := SetDisplay(args[2])
					if err != nil {
						return err
					}
					Wall.Set()
					return nil
				} else {
					Wall.Set()
				}
			} else {
				log.Fatal("Invalid Path")
			}
			return nil
		}
	},
}

// var StatisticsCmd = &Z.Cmd{
// 	Name:     `statistics`,
// 	Commands: []*Z.Cmd{help.Cmd},
// 	Aliases:  []string{`stats`, `bydeath`},
// 	Summary:  `display statistics of a **match** by **death type**`,
// 	Call: func(x *Z.Cmd, args ...string) error {
// 		if len(args) == 0 {
// 			return x.UsageError()
// 		}
// 		// always use "log" and not "fmt" for errors and debugging
// 		log.Printf("Stats of match << %v >> (by death type)", args[0])

// 		// Parse as int
// 		n, err := strconv.Atoi(args[0])
// 		if err != nil {
// 			log.Fatal(err)
// 		}

// 		if n >= 1 {
// 			_ = g.GameOutputStatistic(n)
// 		} else {
// 			log.Fatal("Invalid Match number. Must be greater or equal to 1.")
// 		}
// 		return nil
// 	},
// }

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
		Wall.Set()
		return nil
	},
}

func SetDisplay(display string) error {
	log.Printf("Setting wallpaper to dimension-position of: %v", display)
	dimension := display

	if validDimension(dimension) {
		size, err := parseSize(dimension)
		if err != nil {
			return err
		}
		Wall.Config.Dimensions = size
		return nil
	} else {
		log.Fatal("Dimension parameter is incorrect")
	}

	return nil
}
