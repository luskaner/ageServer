package internal

import (
	"net"
	"testing"
)

func TestFlagsCheck(t *testing.T) {
	cases := []struct {
		name  string
		flags net.Flags
		want  bool
	}{
		{"up+broadcast", net.FlagUp | net.FlagBroadcast, true},
		{"up only", net.FlagUp, false},
		{"broadcast only", net.FlagBroadcast, false},
		{"loopback excluded", net.FlagUp | net.FlagBroadcast | net.FlagLoopback, false},
		{"none", 0, false},
		{"all except loopback", net.FlagUp | net.FlagBroadcast | net.FlagMulticast, true},
	}
	for _, tc := range cases {
		if got := FlagsCheck(tc.flags); got != tc.want {
			t.Errorf("%s: FlagsCheck(%v) = %v, want %v", tc.name, tc.flags, got, tc.want)
		}
	}
}
