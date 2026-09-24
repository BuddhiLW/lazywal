package shell

import "testing"

func TestStatStartTime(t *testing.T) {
	const rest = " R 347320 347322 347320 0 -1 4194304 472 0 0 0 0 0 0 0 20 0 1 0 654137 16646144 1829"
	tests := []struct {
		name, stat, want string
	}{
		{"plain name", "347322 (cat)" + rest, "654137"},
		{"name with spaces and parens", "347322 (my) (pro gram))" + rest, "654137"},
		{"name that looks like fields", "347322 (x) R 1 2 3)" + rest, "654137"},
		{"truncated", "347322 (cat) R 347320 347322", ""},
		{"no name", "347322 cat" + rest, ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statStartTime(tt.stat); got != tt.want {
				t.Errorf("statStartTime(%q) = %q, want %q", tt.stat, got, tt.want)
			}
		})
	}
}
