package loop

import "log"

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
