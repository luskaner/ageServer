package router

import (
	"net/http/httptest"
	"testing"
)

// TestProxyCheck_httptest cubre la lógica de dispatch por Host del Proxy: con
// puerto, sin puerto, mayúsculas y host no coincidente.
func TestProxyCheck_httptest(t *testing.T) {
	p := &Proxy{host: "cdn.ageofempires.com"}

	cases := []struct {
		name string
		host string
		want bool
	}{
		{"exact", "cdn.ageofempires.com", true},
		{"withPort", "cdn.ageofempires.com:443", true},
		{"caseInsensitive", "CDN.AGEOFEMPIRES.COM", true},
		{"wrongDomain", "api.ageofempires.com", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tc.host
			if got := p.Check(req); got != tc.want {
				t.Fatalf("Check(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

// TestProxyName_httptest verifica el nombre del proxy.
func TestProxyName_httptest(t *testing.T) {
	p := &Proxy{host: "cdn.ageofempires.com"}
	if got := p.Name(); got != "proxy cdn.ageofempires.com" {
		t.Fatalf("Name() = %q", got)
	}
}

// TestPlayfabApiCheckAndName_httptest cubre el dispatch por Host de PlayfabApi
// (solo hosts bajo *.playfabapi.com) y su nombre.
func TestPlayfabApiCheckAndName_httptest(t *testing.T) {
	p := &PlayfabApi{}

	cases := []struct {
		name string
		host string
		want bool
	}{
		{"playfabSubdomain", "ed603.playfabapi.com", true},
		{"otherPlayfabSubdomain", "c15f9.playfabapi.com", true},
		{"wrongDomain", "age2.example.com", false},
		{"cdn", "cdn.ageofempires.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tc.host
			if got := p.Check(req); got != tc.want {
				t.Fatalf("Check(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}

	if p.Name() != "playfabapi" {
		t.Fatalf("Name() = %q, want playfabapi", p.Name())
	}
}
