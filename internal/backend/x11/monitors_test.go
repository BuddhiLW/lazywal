package x11_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/backend/x11"
	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
)

const twoMonitors = `Screen 0: minimum 320 x 200, current 4480 x 1200, maximum 16384 x 16384
eDP-1 connected primary 1920x1200+0+0 (normal left inverted right x axis y axis) 344mm x 215mm
   1920x1200     60.00*
HDMI-1-0 connected 2560x1080+1920+0 (normal left inverted right x axis y axis) 673mm x 284mm
   2560x1080     60.00*+  74.99`

var twoMonitorsParsed = []x11.Monitor{
	{Name: "eDP-1", Width: 1920, Height: 1200, X: 0, Y: 0, Primary: true},
	{Name: "HDMI-1-0", Width: 2560, Height: 1080, X: 1920, Y: 0},
}

func TestParseXrandr(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    []x11.Monitor
		wantErr bool
	}{
		{
			name: "laptop and ultrawide",
			out:  twoMonitors,
			want: twoMonitorsParsed,
		},
		{
			name: "disconnected output is skipped",
			out: `Screen 0: minimum 320 x 200, current 1920 x 1080, maximum 16384 x 16384
eDP-1 connected primary 1920x1080+0+0 (normal left inverted right x axis y axis) 344mm x 194mm
   1920x1080     60.00*+
HDMI-1 disconnected (normal left inverted right x axis y axis)`,
			want: []x11.Monitor{{Name: "eDP-1", Width: 1920, Height: 1080, Primary: true}},
		},
		{
			name: "connected but inactive output is skipped",
			out: `Screen 0: minimum 320 x 200, current 1080 x 1920, maximum 16384 x 16384
DP-1 connected primary (normal left inverted right x axis y axis)
   2560x1440     59.95 +
DP-2 connected 1080x1920+0+0 left (normal left inverted right x axis y axis) 527mm x 296mm
   1920x1080     60.00*+`,
			want: []x11.Monitor{{Name: "DP-2", Width: 1080, Height: 1920}},
		},
		{
			name:    "no monitors",
			out:     "no connected monitors",
			wantErr: true,
		},
		{
			name: "only disconnected or inactive outputs",
			out: `Screen 0: minimum 320 x 200, current 320 x 200, maximum 16384 x 16384
HDMI-1 disconnected (normal left inverted right x axis y axis)
DP-1 connected (normal left inverted right x axis y axis)
   2560x1440     59.95 +`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := x11.ParseXrandr(tt.out)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseXrandr() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseXrandr() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestXrandrRunsXrandrCurrent(t *testing.T) {
	fake := shelltest.New()
	fake.Outputs[shell.XrandrCurrent().String()] = twoMonitors

	got, err := x11.Xrandr{Runner: fake}.Monitors()
	if err != nil {
		t.Fatalf("Monitors() = %v", err)
	}
	if !reflect.DeepEqual(got, twoMonitorsParsed) {
		t.Errorf("Monitors() = %+v, want %+v", got, twoMonitorsParsed)
	}
	if !fake.Ran("xrandr --current") {
		t.Errorf("calls = %q, want xrandr --current", fake.Commands())
	}
}

func TestXrandrFailure(t *testing.T) {
	fake := shelltest.New()
	boom := errors.New("cannot open display")
	fake.Errors["xrandr"] = boom

	if _, err := (x11.Xrandr{Runner: fake}).Monitors(); !errors.Is(err, boom) {
		t.Errorf("Monitors() = %v, want wrapped %v", err, boom)
	}
}
