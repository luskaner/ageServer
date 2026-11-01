package server

import (
	"net"
	"testing"
)

func TestCalculateBroadcastIPv4(t *testing.T) {
	tests := []struct {
		ip   string
		mask net.IPMask
		want string
	}{
		{"192.168.1.100", net.CIDRMask(24, 32), "192.168.1.255"},
		{"10.0.0.50", net.CIDRMask(8, 32), "10.255.255.255"},
		{"172.16.0.1", net.CIDRMask(16, 32), "172.16.255.255"},
		{"192.168.1.100", net.CIDRMask(17, 32), "192.168.127.255"},
		{"0.0.0.0", net.CIDRMask(0, 32), "255.255.255.255"},
		{"192.168.1.100", net.CIDRMask(32, 32), "192.168.1.100"},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.ip).To4()
		got := calculateBroadcastIPv4(ip, tt.mask)
		if got.String() != tt.want {
			t.Errorf("calculateBroadcastIPv4(%s, %v) = %s, want %s", tt.ip, tt.mask, got, tt.want)
		}
	}
}
