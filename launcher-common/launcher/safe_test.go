package launcher

import "testing"

func TestSafeAppliesToClient(t *testing.T) {
	cases := []struct {
		name     string
		mode     string
		official bool
		want     bool
	}{
		{"auto official", "auto", true, true},
		{"auto custom", "auto", false, false},
		{"true official", "true", true, true},
		{"true custom", "true", false, true},
		{"false official", "false", true, false},
		{"false custom", "false", false, false},
		{"old server", "", true, false},
		{"unknown mode", "sometimes", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SafeAppliesToClient(tc.mode, tc.official); got != tc.want {
				t.Fatalf("SafeAppliesToClient(%q, %v) = %v, want %v", tc.mode, tc.official, got, tc.want)
			}
		})
	}
}
