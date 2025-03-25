package loop

import "fmt"

func SetDisplay(display string) error {
	dimension := display

	if !validDimension(dimension) {
		return fmt.Errorf("invalid dimension parameter: %s", dimension)
	}

	size, err := parseSize(dimension)
	if err != nil {
		return err
	}

	Wall.Config.Dimensions = size
	return nil
}
