package launcher

import (
	"net/netip"
	"testing"

	"github.com/luskaner/ageServer/common"
)

// The embedded API is the default, so a launcher with no configuration at all
// keeps doing what it did before this option existed.
func TestServerApiHostDefaultsToEmbedded(t *testing.T) {
	got, ok := defaultLayers()["Server.Api.Host"]
	if !ok {
		t.Fatal("Server.Api.Host has no default")
	}
	if want := netip.IPv4Unspecified().String(); got != want {
		t.Fatalf("Server.Api.Host default = %v, want %q", got, want)
	}
	if got := flagBindings()["serverApi"]; got != "Server.Api.Host" {
		t.Fatalf("serverApi is bound to %q, want Server.Api.Host", got)
	}
}

func TestServerApiSeparate(t *testing.T) {
	for _, tc := range []struct {
		host string
		want bool
	}{
		{"", false},
		{"0.0.0.0", false},
		{" 0.0.0.0 ", false},
		{"192.168.1.20", true},
		{"api.example.com", true},
	} {
		if got := ServerApiSeparate(tc.host); got != tc.want {
			t.Errorf("ServerApiSeparate(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestValidateApiHostValue(t *testing.T) {
	cases := []struct {
		value string
		want  int
	}{
		{netip.IPv4Unspecified().String(), common.ErrSuccess},
		{"192.168.1.20", common.ErrSuccess},
		{"api.example.com", common.ErrSuccess},
		{" 192.168.1.20 ", common.ErrSuccess},
		// Empty means the embedded API, like the default.
		{"", common.ErrSuccess},
		{"   ", common.ErrSuccess},
		{"::1", ErrInvalidServerApiHost},
		{"fe80::1", ErrInvalidServerApiHost},
	}
	for _, tc := range cases {
		if got := ValidateApiHostValue(Discard{}, tc.value); got != tc.want {
			t.Errorf("ValidateApiHostValue(%q) = %d, want %d", tc.value, got, tc.want)
		}
	}
}
