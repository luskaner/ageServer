package cmd

import "testing"

// Regression: the previous inline loop selected the LAST suitable IP because
// it never broke after a match, making the choice dependent on the interface
// enumeration order.
func TestSelectNonLoopbackIPFirstMatchWins(t *testing.T) {
	got := selectNonLoopbackIP([]string{"127.0.0.1", "192.168.1.2", "10.0.0.3"})
	if got != "192.168.1.2" {
		t.Fatalf("got %q, want first non-loopback %q", got, "192.168.1.2")
	}
}

func TestSelectNonLoopbackIPCases(t *testing.T) {
	cases := []struct {
		name string
		ips  []string
		want string
	}{
		{"all loopback", []string{"127.0.0.1", "::1"}, ""},
		{"empty", nil, ""},
		{"invalid entries skipped", []string{"garbage", "10.1.2.3"}, "10.1.2.3"},
		{"single suitable", []string{"10.0.0.9"}, "10.0.0.9"},
	}
	for _, tc := range cases {
		if got := selectNonLoopbackIP(tc.ips); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
