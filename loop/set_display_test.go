package loop

import "testing"

func TestSetDisplay(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		expectedDim Size
	}{
		{
			name:    "valid dimension",
			input:   "1920x1080",
			wantErr: false,
			expectedDim: Size{
				Width:  1920.0,
				Height: 1080.0,
			},
		},
		{
			name:        "invalid format",
			input:       "1920-1080",
			wantErr:     true,
			expectedDim: Size{},
		},
		{
			name:        "empty string",
			input:       "",
			wantErr:     true,
			expectedDim: Size{},
		},
		{
			name:        "invalid numbers",
			input:       "abcxdef",
			wantErr:     true,
			expectedDim: Size{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Wall.Config = &Config{}
			err := SetDisplay(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("SetDisplay() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if *Wall.Config.Dimensions != tt.expectedDim {
					t.Errorf("SetDisplay() dimensions = %v, want %v",
						*Wall.Config.Dimensions, tt.expectedDim)
				}
			}
		})
	}
}
