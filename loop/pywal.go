package loop

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func getDuration(inputVideo string) (string, error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("ffmpeg -i %s 2>&1 | grep Duration", inputVideo))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	outputStr := string(output)

	durationIndex := strings.Index(outputStr, "Duration: ")
	if durationIndex == -1 {
		return "", fmt.Errorf("could not find duration in ffmpeg output")
	}

	durationStr := outputStr[durationIndex+10 : durationIndex+21]
	return durationStr, nil
}

func convertToSeconds(duration string) (int, error) {
	parts := strings.Split(duration, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid duration format")
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}

	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}

	seconds, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, err
	}

	totalSeconds := hours*3600 + minutes*60 + int(seconds)
	return totalSeconds, nil
}

func formatTime(seconds int) string {
	return fmt.Sprintf("%02d:%02d:%02d", seconds/3600, (seconds%3600)/60, seconds%60)
}

func extractFrame(inputVideo, outputImage, time string) error {
	if validPath(outputImage) {
		os.Remove(outputImage)
	}

	cmd := exec.Command("ffmpeg", "-ss", time, "-i", inputVideo, "-frames:v", "1", "-q:v", "2", outputImage)
	return cmd.Run()
}

func UpdatePywalScheme(image string) error {
	cmd := exec.Command("bash", "-c", fmt.Sprintf("wal -n -i %s", image))
	return cmd.Run()
}

func (w *Wallpaper) Pywal() {
	rand.Seed(time.Now().UnixNano())

	inputVideo := w.Config.Path
	outputImage := fmt.Sprintf("./%s.png", uuid.New().String())

	duration, err := getDuration(inputVideo)
	if err != nil {
		fmt.Println("Error getting duration:", err)
		return
	}

	durationSeconds, err := convertToSeconds(duration)
	if err != nil {
		fmt.Println("Error converting duration to seconds:", err)
		return
	}

	randomTime := rand.Intn(durationSeconds)
	randomTimeFormatted := formatTime(randomTime)

	fmt.Println("outputing image")
	err = extractFrame(inputVideo, outputImage, randomTimeFormatted)
	if err != nil {
		fmt.Println("Error extracting frame:", err)
		return
	}

	fmt.Printf("Captured frame at %s, saved to %s (temporary)\n", randomTimeFormatted, outputImage)
	err = UpdatePywalScheme(outputImage)
	if err != nil {
		fmt.Println("Error updating Pywal scheme:", err)
		return
	}

	// time.Sleep(1 * time.Second)
	// // Attempt to remove the file
	err = os.Remove(outputImage)
	if err != nil {
		// Handle the error
		fmt.Println("Error deleting temporary file:", err)
	} else {
		fmt.Println("Temporary file deleted successfully")
	}
}
