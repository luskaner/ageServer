package server

import (
	"net"
	"testing"

	"github.com/luskaner/ageServer/common"
	commonServer "github.com/luskaner/ageServer/common/server"
)

func measured(ip string, mode common.CertMode) MesuredIpAddress {
	data := &commonServer.AnnounceMessageDataSupportedLatest{GameTitle: "age2", Version: "2", Mode: string(mode)}
	return MesuredIpAddress{
		Ip:   net.ParseIP(ip),
		Mode: string(mode),
		Data: data,
	}
}

func TestSelectServerIPPrefersTheGameClass(t *testing.T) {
	addrs := []MesuredIpAddress{
		measured("192.168.1.10", common.CertModeModern),
		measured("192.168.1.17", common.CertModeLegacy),
	}
	selected, ok := SelectServerIP("age2", addrs, nil)
	if !ok || selected.Ip.String() != "192.168.1.17" {
		t.Fatalf("age2 selected %v, want the legacy address", selected.Ip)
	}
	selected, ok = SelectServerIP("age4", addrs, nil)
	if !ok || selected.Ip.String() != "192.168.1.10" {
		t.Fatalf("age4 selected %v, want the modern address", selected.Ip)
	}
}

func TestSelectServerIPOldServerAndFallback(t *testing.T) {
	old := MesuredIpAddress{Ip: net.ParseIP("192.168.1.5"), Data: &commonServer.AnnounceMessageDataSupportedLatest{GameTitle: "age2", Version: "1"}}
	selected, ok := SelectServerIP("age2", []MesuredIpAddress{old}, func(string) {
		t.Fatal("an old server with no mode must be used without a warning")
	})
	if !ok || selected.Ip.String() != "192.168.1.5" {
		t.Fatalf("selected %v, want the old server", selected.Ip)
	}

	warned := false
	odd := MesuredIpAddress{Ip: net.ParseIP("192.168.1.10"), Data: &commonServer.AnnounceMessageDataSupportedLatest{GameTitle: "age2", Version: "3", Mode: "unknown"}}
	selected, ok = SelectServerIP("age2", []MesuredIpAddress{odd}, func(string) {
		warned = true
	})
	if !ok || !warned || selected.Ip.String() != "192.168.1.10" {
		t.Fatalf("expected a warned fallback, got %v warned=%v", selected.Ip, warned)
	}
	if _, ok = SelectServerIP("age2", nil, nil); ok {
		t.Fatal("no addresses must not resolve")
	}
}
