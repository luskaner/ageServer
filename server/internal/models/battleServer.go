package models

import (
	"fmt"
	"io"
	"iter"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/uuid"
	"github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
)

// externalIPTimeout bounds the lookup of this machine's public address.
//
// It used to use http.DefaultClient, which has no timeout at all, followed by an
// unbounded io.ReadAll. This runs during startup, before the server binds the
// port the launcher polls to decide it came up, and the launcher's budget for
// that is a handful of seconds: it then concludes the server failed and kills it.
// A captive portal, a proxy or a half-open TLS path was therefore enough to get
// the server killed by the launcher on the very machine that made it slow.
const externalIPTimeout = 3 * time.Second

// maxExternalIPBody caps the read so a misbehaving endpoint cannot stream for as
// long as the client timeout happens to allow.
const maxExternalIPBody = 64

// externalIPClient is a var so tests can exercise the lookup without a network.
var externalIPClient = &http.Client{Timeout: externalIPTimeout}

// externalIPURL is a var so tests can point the lookup at a local server.
var externalIPURL = "https://api.ipify.org/"

// localIp returns the address of this machine the request reached us on, which
// is by definition reachable for whoever sent it.
func localIp(r *http.Request) (ip string) {
	addr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return
	}
	var err error
	ip, _, err = net.SplitHostPort(addr.String())
	if err != nil {
		return
	}
	if parsedIP := net.ParseIP(ip); parsedIP != nil && parsedIP.To4() != nil {
		return ip
	}
	return
}

// notRoutablePrefixes are the IPv4 ranges that the net.IP predicates do not
// cover and that, like RFC 1918, cannot be reached from the public internet:
// 100.64.0.0/10 is carrier grade NAT (RFC 6598) and 240.0.0.0/4 is reserved,
// up to and including the limited broadcast address.
var notRoutablePrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

// isNotRoutable reports whether an IPv4 address belongs to a range that can
// neither be reached from nor reach the public internet.
func isNotRoutable(ip4 net.IP) bool {
	if ip4.IsLoopback() || ip4.IsPrivate() || ip4.IsUnspecified() ||
		ip4.IsLinkLocalUnicast() || ip4.IsLinkLocalMulticast() ||
		ip4.IsInterfaceLocalMulticast() || ip4.IsMulticast() {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip4)
	if !ok {
		return false
	}
	for _, prefix := range notRoutablePrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

var localSubnets []*net.IPNet
var publicIp string

func CacheNetworkInterfaces(externalIPAddress string) {
	if externalIPAddress != "auto" {
		if ip := net.ParseIP(externalIPAddress); ip != nil && ip.To4() != nil {
			publicIp = externalIPAddress
		}
	} else if internal.CanUseInternet {
		// Bounded on purpose: see externalIPTimeout. Failing here is fine, the
		// public address is only used to pick subnets for LAN broadcast.
		if resp, err := externalIPClient.Get(externalIPURL); err == nil {
			defer func(Body io.ReadCloser) {
				_ = Body.Close()
			}(resp.Body)
			if ipBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxExternalIPBody)); err == nil {
				ipStr := string(ipBytes)
				if ip := net.ParseIP(ipStr); ip != nil && ip.To4() != nil {
					publicIp = ipStr
				}
			}
		}
	}
	if publicIp == "" && externalIPAddress == "auto" && internal.CanUseInternet {
		logger.Warn("Could not determine the public IP address, battle servers left as 'auto' will only be reachable from the local networks. Set Internet.IP manually if other networks need to reach them.")
	}
	cacheLocalSubnets()
}

// cacheLocalSubnets records the subnets of the running network interfaces.
//
// It used to run only once a public address had been resolved, which tied two
// unrelated facts together: a blocked, slow or captive-portalled lookup left the
// list empty, and then every peer, the ones on the local networks included, was
// mistaken for a remote one.
func cacheLocalSubnets() {
	localSubnets = nil
	ifs, err := common.RunningNetworkInterfaces()
	if err != nil {
		return
	}
	for _, ipNets := range ifs {
		localSubnets = append(localSubnets, ipNets...)
	}
}

type BattleServer interface {
	SetLAN(lan bool)
	SetIPv4(ipv4 string)
	SetBsPort(bsPort int)
	SetWebSocketPort(webSocketPort int)
	SetOutOfBandPort(outOfBandPort int)
	SetHasOobPort(hasOobPort bool)
	SetBattleServerName(battleServerName string)
	SetName(name string)
	LAN() bool
	Region() string
	AppendName(encoded *internal.A)
	EncodeLogin(r *http.Request) internal.A
	EncodePorts() internal.A
	EncodeAdvertisement(r *http.Request) internal.A
	ResolveIPv4(r *http.Request) string
	ResolveIPv4ForLocalIP(localIP string) string
	String() string
}

type MainBattleServer struct {
	battleServer.Base `koanf:",squash"`
	lan               *bool
	hasOobPort        bool
	battleServerName  string
	lanMu             sync.RWMutex
}

func (battleServer *MainBattleServer) SetBattleServerName(battleServerName string) {
	battleServer.battleServerName = battleServerName
}

func (battleServer *MainBattleServer) SetHasOobPort(hasOobPort bool) {
	battleServer.hasOobPort = hasOobPort
}

func (battleServer *MainBattleServer) SetIPv4(ipv4 string) {
	battleServer.IPv4 = ipv4
}

func (battleServer *MainBattleServer) SetBsPort(bsPort int) {
	battleServer.BsPort = bsPort
}

func (battleServer *MainBattleServer) SetWebSocketPort(webSocketPort int) {
	battleServer.WebSocketPort = webSocketPort
}

func (battleServer *MainBattleServer) SetOutOfBandPort(outOfBandPort int) {
	battleServer.OutOfBandPort = outOfBandPort
}

func (battleServer *MainBattleServer) SetName(name string) {
	battleServer.Name = name
}

func (battleServer *MainBattleServer) LAN() bool {
	battleServer.lanMu.RLock()
	if battleServer.lan == nil {
		battleServer.lanMu.RUnlock()
		var lan bool
		battleServer.lanMu.Lock()
		battleServer.lan = &lan
		defer battleServer.lanMu.Unlock()
		if _, err := uuid.Parse(battleServer.Base.Region); err == nil {
			lan = true
		}
	} else {
		defer battleServer.lanMu.RUnlock()
	}
	return *battleServer.lan
}

func (battleServer *MainBattleServer) AppendName(encoded *internal.A) {
	switch battleServer.battleServerName {
	case "omit":
	case "null":
		*encoded = append(*encoded, nil)
	default:
		*encoded = append(*encoded, battleServer.Name)
	}
}

func (battleServer *MainBattleServer) SetLAN(enable bool) {
	battleServer.lanMu.Lock()
	defer battleServer.lanMu.Unlock()
	battleServer.lan = &enable
}

func (battleServer *MainBattleServer) Region() string {
	return battleServer.Base.Region
}

func (battleServer *MainBattleServer) EncodeLogin(r *http.Request) internal.A {
	encoded := internal.A{
		battleServer.Base.Region,
	}
	battleServer.AppendName(&encoded)
	encoded = append(encoded, battleServer.ResolveIPv4(r))
	encoded = append(encoded, battleServer.EncodePorts()...)
	return encoded
}

func (battleServer *MainBattleServer) EncodePorts() internal.A {
	encoded := internal.A{battleServer.BsPort}
	encoded = append(encoded, battleServer.WebSocketPort)
	if battleServer.hasOobPort {
		encoded = append(encoded, battleServer.OutOfBandPort)
	}
	return encoded
}

func (battleServer *MainBattleServer) EncodeAdvertisement(r *http.Request) internal.A {
	encoded := internal.A{
		battleServer.ResolveIPv4(r),
	}
	encoded = append(encoded, battleServer.EncodePorts()...)
	return encoded
}

func (battleServer *MainBattleServer) ResolveIPv4(r *http.Request) (ipV4 string) {
	if battleServer.IPv4 != "auto" {
		return battleServer.IPv4
	}
	// Whatever else happens, the address the peer used to reach us is an address
	// it can reach us back on, so it is the answer of last resort. Returning an
	// empty one instead leaves the game stalled on it with nothing to report.
	ipV4 = localIp(r)
	remoteIPStr, _, _ := net.SplitHostPort(r.RemoteAddr)
	remoteIP := net.ParseIP(remoteIPStr)
	if remoteIP == nil || remoteIP.To4() == nil || !internal.CanUseInternet {
		return
	}
	remoteIP4 := remoteIP.To4()
	for _, subnet := range localSubnets {
		if subnet.Contains(remoteIP4) {
			return
		}
	}
	// Sitting on one of our own subnets is only one way of being local: a peer
	// can be on a different and still private one, reaching us through a router,
	// a second access point or a guest network. It did not come from the public
	// internet, so handing it our public address would leave it unable to
	// connect at all.
	if isNotRoutable(remoteIP4) {
		return
	}
	host := r.Host
	if strings.Contains(host, ":") {
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
	}
	if ip := net.ParseIP(host); ip != nil && ip.To4() != nil {
		ipV4 = host
	} else if publicIp != "" {
		ipV4 = publicIp
	}
	return
}

func (battleServer *MainBattleServer) ResolveIPv4ForLocalIP(localIP string) string {
	if battleServer.IPv4 != "auto" {
		return battleServer.IPv4
	}
	ip := net.ParseIP(localIP)
	if ip == nil || ip.To4() == nil {
		return ""
	}
	return localIP
}

func (battleServer *MainBattleServer) String() string {
	str := fmt.Sprintf(
		"Region: %s (Name: %s), IPv4: %s, Ports: ",
		battleServer.Base.Region,
		battleServer.Name,
		battleServer.IPv4,
	)
	ports := battleServer.EncodePorts()
	str += fmt.Sprintf("%v", ports)
	return str
}

type BattleServers interface {
	Initialize(battleServers []BattleServer, opts *BattleServerOpts)
	Iter() iter.Seq2[string, BattleServer]
	Encode(r *http.Request) internal.A
	Get(region string) (BattleServer, bool)
	NewLANBattleServer(region string) BattleServer
	NewBattleServer(region string) BattleServer
}

type BattleServerOpts struct {
	OobPort bool
	Name    string
}

type MainBattleServers struct {
	store            *internal.ReadOnlyOrderedMap[string, BattleServer]
	haveOobPort      bool
	battleServerName string
}

func (battleSrvs *MainBattleServers) Initialize(battleServers []BattleServer, opts *BattleServerOpts) {
	if opts == nil {
		opts = &BattleServerOpts{
			OobPort: true,
		}
	}
	if opts.Name == "" {
		opts.Name = "true"
	}
	keyOrder := make([]string, len(battleServers))
	mapping := make(map[string]BattleServer, len(battleServers))
	for i, bs := range battleServers {
		battleServers[i].SetHasOobPort(opts.OobPort)
		battleServers[i].SetBattleServerName(opts.Name)
		keyOrder[i] = bs.Region()
		mapping[keyOrder[i]] = battleServers[i]
	}
	battleSrvs.battleServerName = opts.Name
	battleSrvs.haveOobPort = opts.OobPort
	battleSrvs.store = internal.NewReadOnlyOrderedMap[string, BattleServer](keyOrder, mapping)
}

func (battleSrvs *MainBattleServers) Iter() iter.Seq2[string, BattleServer] {
	return battleSrvs.store.Iter()
}

func (battleSrvs *MainBattleServers) Encode(r *http.Request) internal.A {
	encoded := make(internal.A, battleSrvs.store.Len())
	i := 0
	for _, bs := range battleSrvs.store.Iter() {
		encoded[i] = bs.EncodeLogin(r)
		i++
	}
	return encoded
}

func (battleSrvs *MainBattleServers) Get(region string) (BattleServer, bool) {
	return battleSrvs.store.Load(region)
}

func (battleSrvs *MainBattleServers) NewLANBattleServer(region string) BattleServer {
	bs := battleSrvs.NewBattleServer(region)
	bs.SetLAN(true)
	return bs
}

func (battleSrvs *MainBattleServers) NewBattleServer(region string) BattleServer {
	return &MainBattleServer{
		Base: battleServer.Base{
			Region: region,
		},
		hasOobPort:       battleSrvs.haveOobPort,
		battleServerName: battleSrvs.battleServerName,
	}
}
