package server

import (
	"net/netip"

	mapset "github.com/deckarep/golang-set/v3"
)

const AnnounceIdLength = 36

type AnnounceMessage struct {
	IpAddrs mapset.Set[netip.Addr]
}
