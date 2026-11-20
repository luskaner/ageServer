package login

import "testing"

func TestLoginIsOfficial(t *testing.T) {
	cases := []struct {
		name             string
		xbox             bool
		steamTicketPlain bool
		wantOfficial     bool
	}{
		{"xbox is always official", true, false, true},
		{"xbox ignores a plaintext steam flag", true, true, true},
		{"steam with a real (encrypted) ticket is official", false, false, true},
		{"steam with a plaintext ticket is not official", false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := loginIsOfficial(tc.xbox, tc.steamTicketPlain); got != tc.wantOfficial {
				t.Fatalf("loginIsOfficial(%v, %v) = %v, want %v",
					tc.xbox, tc.steamTicketPlain, got, tc.wantOfficial)
			}
		})
	}
}
