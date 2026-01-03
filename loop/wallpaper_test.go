package loop

import (
	"os/exec"
	"testing"
	"time"
)

func TestGetMonitors(t *testing.T) {
	tests := []struct {
		name       string
		mockXrandr string
		want       []Monitor
		wantErr    bool
	}{
		{
			name: "laptop and ultrawide",
			mockXrandr: `Screen 0: minimum 320 x 200, current 4480 x 1200, maximum 16384 x 16384
eDP-1 connected primary 1920x1200+0+0 (normal left inverted right x axis y axis) 344mm x 215mm
   1920x1200     60.00* 
HDMI-1-0 connected 2560x1080+1920+0 (normal left inverted right x axis y axis) 673mm x 284mm
   2560x1080     60.00*+  74.99`,
			want: []Monitor{
				{
					Name:    "eDP-1",
					Primary: true,
					Dimensions: Size{
						Width:  1920,
						Height: 1200,
					},
					Position: Position{
						X: 0,
						Y: 0,
					},
				},
				{
					Name:    "HDMI-1-0",
					Primary: false,
					Dimensions: Size{
						Width:  2560,
						Height: 1080,
					},
					Position: Position{
						X: 1920,
						Y: 0,
					},
				},
			},
			wantErr: false,
		},
		{
			name:       "no monitors",
			mockXrandr: "no connected monitors",
			want:       nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldExec := execCommand
			execCommand = func(name string, args ...string) *exec.Cmd {
				return testMockCommand(tt.mockXrandr)
			}
			defer func() { execCommand = oldExec }()

			got, err := GetMonitors()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMonitors() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("GetMonitors() got %v monitors, want %v", len(got), len(tt.want))
					return
				}

				for i, monitor := range got {
					if monitor.Name != tt.want[i].Name {
						t.Errorf("Monitor[%d].Name = %v, want %v", i, monitor.Name, tt.want[i].Name)
					}
					if monitor.Primary != tt.want[i].Primary {
						t.Errorf("Monitor[%d].Primary = %v, want %v", i, monitor.Primary, tt.want[i].Primary)
					}
					if monitor.Dimensions != tt.want[i].Dimensions {
						t.Errorf("Monitor[%d].Dimensions = %v, want %v", i, monitor.Dimensions, tt.want[i].Dimensions)
					}
					if monitor.Position != tt.want[i].Position {
						t.Errorf("Monitor[%d].Position = %v, want %v", i, monitor.Position, tt.want[i].Position)
					}
				}
			}
		})
	}
}

func TestStartOnMonitor(t *testing.T) {
	tests := []struct {
		name       string
		monitor    Monitor
		videoPath  string
		mockOutput string
		wantErr    bool
	}{
		{
			name: "laptop display",
			monitor: Monitor{
				Name: "eDP-1",
				Dimensions: Size{
					Width:  1920,
					Height: 1200,
				},
				Position: Position{
					X: 0,
					Y: 0,
				},
				Primary: true,
			},
			videoPath:  "../test/wallpaper.mp4",
			mockOutput: "Started MPV...",
			wantErr:    false,
		},
		{
			name: "ultrawide monitor",
			monitor: Monitor{
				Name: "HDMI-1-0",
				Dimensions: Size{
					Width:  2560,
					Height: 1080,
				},
				Position: Position{
					X: 1920,
					Y: 0,
				},
			},
			videoPath:  "../test/wallpaper.mp4",
			mockOutput: "Started MPV...",
			wantErr:    false,
		},
		// Skipping command failure test - mocking exec.Command.Start() is complex
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Wallpaper{
				Config: &Config{
					Path: tt.videoPath,
				},
				Running: make(map[string]*exec.Cmd),
			}

			oldExec := execCommand
			execCommand = func(name string, args ...string) *exec.Cmd {
				return testMockCommand(tt.mockOutput)
			}
			defer func() { execCommand = oldExec }()

			err := w.startOnMonitor(tt.monitor)
			if (err != nil) != tt.wantErr {
				t.Errorf("startOnMonitor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if cmd, ok := w.Running[tt.monitor.Name]; !ok || cmd == nil {
					t.Errorf("Process not tracked for monitor %s", tt.monitor.Name)
				}
			}

			time.Sleep(100 * time.Millisecond) // Wait for goroutines
		})
	}
}

func TestWallpaperSet(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid video file",
			config: &Config{
				Path: "../test/wallpaper.mp4",
				Dimensions: &Size{
					Width:  1920,
					Height: 1080,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWallPaper(tt.config)
			err := w.Set()
			if (err != nil) != tt.wantErr {
				t.Errorf("Wallpaper.Set() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify processes were started for each monitor
			if !tt.wantErr {
				if len(w.Running) == 0 {
					t.Error("No processes started")
				}
				for monitor, cmd := range w.Running {
					if cmd == nil || cmd.Process == nil {
						t.Errorf("Process not started for monitor %s", monitor)
					}
				}
			}
		})
	}
}
