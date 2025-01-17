package dependencies_test

import (
	"errors"
	"fmt"
	"testing"
)

type MockLookPath map[string]string

func (m MockLookPath) Look(binary string) (string, error) {
	if value, exists := m[binary]; exists {
		return value, nil
	}
	return "", errors.New("not found")
}

func TestMockLookPath(t *testing.T) {
	mockLookPath := MockLookPath{
		"mpv":      "/mock/path/mpv",
		"xwinwrap": "/mock/path/xwinwrap",
	}

	t.Run("BinaryExists", func(t *testing.T) {
		path, err := mockLookPath.Look("mpv")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if path != "/mock/path/mpv" {
			t.Errorf("Expected '/mock/path/mpv', got: %s", path)
		}
	})

	t.Run("BinaryDoesNotExist", func(t *testing.T) {
		_, err := mockLookPath.Look("ffmpeg")
		if err == nil {
			t.Error("Expected error for missing binary, got nil")
		}
	})
}

func TestDetermineMissingDependencies(t *testing.T) {
	mockLookPath := MockLookPath{
		"mpv":      "/mock/path/mpv",
		"xwinwrap": "/mock/path/xwinwrap",
	}

	t.Run("AllBinariesPresent", func(t *testing.T) {
		binaries := []string{"mpv", "xwinwrap"}
		var missing string

		for _, binary := range binaries {
			_, err := mockLookPath.Look(binary)
			if err != nil {
				if missing == "" {
					missing = binary
				} else {
					missing = fmt.Sprintf("%s, %s", missing, binary)
				}
			}
		}

		if missing != "" {
			t.Errorf("Expected no missing binaries, got: %s", missing)
		}
	})

	t.Run("SomeBinariesMissing", func(t *testing.T) {
		binaries := []string{"mpv", "xwinwrap", "ffmpeg", "wal"}
		var missing string

		for _, binary := range binaries {
			_, err := mockLookPath.Look(binary)
			if err != nil {
				if missing == "" {
					missing = binary
				} else {
					missing = fmt.Sprintf("%s, %s", missing, binary)
				}
			}
		}

		expectedMissing := "ffmpeg, wal"
		if missing != expectedMissing {
			t.Errorf("Expected missing binaries to be '%s', got '%s'", expectedMissing, missing)
		}
	})

	t.Run("AllBinariesMissing", func(t *testing.T) {
		binaries := []string{"ffmpeg", "wal"}
		var missing string

		for _, binary := range binaries {
			_, err := mockLookPath.Look(binary)
			if err != nil {
				if missing == "" {
					missing = binary
				} else {
					missing = fmt.Sprintf("%s, %s", missing, binary)
				}
			}
		}

		expectedMissing := "ffmpeg, wal"
		if missing != expectedMissing {
			t.Errorf("Expected missing binaries to be '%s', got '%s'", expectedMissing, missing)
		}
	})
}
