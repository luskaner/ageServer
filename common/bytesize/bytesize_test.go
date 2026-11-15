package bytesize

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"1024", 1024},
		{" 512 ", 512},
		{"1B", 1},
		{"1b", 1},
		{"1KB", 1 << 10},
		{"1kb", 1 << 10},
		{"1K", 1 << 10},
		{"32MB", 32 << 20},
		{"32M", 32 << 20},
		{"1GB", 1 << 30},
		{"1G", 1 << 30},
		{" 32 MB ", 32 << 20},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"", "   ", "abc", "1XB", "MB", "1.5MB"} {
		if got, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %d, want an error", in, got)
		}
	}
}
