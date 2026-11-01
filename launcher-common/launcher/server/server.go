package server

import (
	"net"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/luskaner/ageServer/common/uuid"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd"
	cmdServer "github.com/luskaner/ageServer/common/cmd/server"
	"github.com/luskaner/ageServer/common/executables"
	commonExecutor "github.com/luskaner/ageServer/common/executor/exec"
	commonProcess "github.com/luskaner/ageServer/common/process"
	"github.com/luskaner/ageServer/common/server"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/serverKill"
	"github.com/spf13/pflag"
	"golang.org/x/net/ipv4"
)

type MesuredIpAddress struct {
	Ip      net.IP
	Latency time.Duration
}

func StartServer(r launcher.Reporter, gameTitle string, stop string, executable string, flags *pflag.FlagSet, values *cmdServer.Values, optionsFn func(options commonExecutor.Options)) (result *commonExecutor.Result, executablePath string, ip string) {
	executablePath = GetExecutablePath(executable)
	if executablePath == "" {
		return
	}
	var showWindow bool
	if stop == "true" {
		showWindow = false
	} else {
		showWindow = true
	}
	options := commonExecutor.Options{File: executablePath, Args: cmd.FlagSetToArgs(flags, false), ShowWindow: showWindow, Pid: true}
	optionsFn(options)
	result = options.Exec()
	if result.Success() {
		localIPs := common.NetIPSliceToNetIPSet(common.StringSliceToNetIPSlice(common.HostOrIpToIps(netip.IPv4Unspecified().String())))
		timeout := time.After(time.Duration(localIPs.Cardinality()) * (server.LatencyMeasurementCount + 1) * time.Second)
	loop:
		for {
			select {
			case <-timeout:
				break loop
			default:
				if _, measuredIpAddrs, data := FilterServerIPs(uuid.MustParse(values.Id), "", gameTitle, localIPs); data != nil {
					ip = measuredIpAddrs[0].Ip.String()
					return
				}
				if _, err := commonProcess.FindProcess(int(result.Pid)); err != nil {
					break loop
				}
			}
		}
		if _, proc, err := commonProcess.Process(executablePath); err == nil && proc != nil {
			if err = serverKill.Do(executablePath); err != nil {
				r.Fail("Failed to stop server")
				r.Fault("Error message: %s", err.Error())
				r.Fault("You may try killing it manually. Kill process server in your task manager.")
			}
		}
		result = nil
	}
	return
}

func GenerateServerCertificates(r launcher.Reporter, serverExecutablePath string, canTrustCertificate bool) (exitCode int) {
	certificateFolder := common.CertificatePairFolder(serverExecutablePath)
	if exists, cert, _, caCert, selfSignedCert, _ := common.CertificatePairs(certificateFolder); !exists || CertificateSoonExpired(cert) || CertificateSoonExpired(caCert) || CertificateSoonExpired(selfSignedCert) {
		if !canTrustCertificate {
			r.Fail("serverStart is true and canTrustCertificate is false. Certificate pair is missing or soon expired. Generate your own certificates manually.")
			exitCode = launcher.ErrServerCertMissingExpired
			return
		}
		if certificateFolder == "" {
			r.Fail("Cannot find certificate folder of the server. Make sure the folder structure of the server is correct.")
			exitCode = launcher.ErrServerCertDirectory
			return
		}
		if result := GenerateCertificatePair(certificateFolder, func(options *commonExecutor.Options) {

		}); !result.Success() {
			r.Fail("Failed to generate certificate pair. Check the folder and its permissions")
			exitCode = launcher.ErrServerCertCreate
			if result != nil {
				if result.Err != nil {
					r.Fault("Error message: %s", result.Err.Error())
				}
				if result.ExitCode != common.ErrSuccess {
					r.Fault("Exit code: %d.", result.ExitCode)
				}
			}
			return
		}
	}
	return
}

func GetExecutablePath(executable string) string {
	if executable == "auto" {
		return executables.FindPath(executables.NativeFileName(true, executables.Server))
	}
	return executable
}

func FilterServerIPs(id uuid.UUID, serverName string, gameTitle string, possibleIpAddrs mapset.Set[netip.Addr]) (actualId uuid.UUID, measuredIpAddresses []MesuredIpAddress, data *server.AnnounceMessageDataSupportedLatest) {
	for ipAddr := range possibleIpAddrs.Iter() {
		ip := common.NetIPAddrToNetIP(ipAddr)
		var ok bool
		var latency time.Duration
		var tmpData *server.AnnounceMessageDataSupportedLatest
		if ok, actualId, latency, tmpData = server.LanServerIP(id, gameTitle, ip, serverName, true, nil, false); ok {
			measuredIpAddresses = append(measuredIpAddresses, MesuredIpAddress{
				Ip:      ip,
				Latency: latency,
			})
			if data == nil {
				data = tmpData
			}
		}
	}
	slices.SortStableFunc(measuredIpAddresses, func(a, b MesuredIpAddress) int {
		return int(a.Latency - b.Latency)
	})
	return
}

// announceRounds is how many times the announce is repeated. A single datagram
// gets lost, so the wait would be unreliable without it, and it is also what the
// progress indicator counts.
const announceRounds = 3

// Progress reports how the discovery is going: round is 1-based out of
// announceRounds, and found is how many distinct 'server's have answered so far.
// It is called from several goroutines at once.
type Progress func(round, rounds, found int)

// QueryServers fills servers with every 'server' that answers the announce.
func QueryServers(
	multicastGroups mapset.Set[netip.Addr],
	targetPorts mapset.Set[uint16],
	servers map[uuid.UUID]*AnnounceMessage,
) {
	QueryServersWithProgress(multicastGroups, targetPorts, servers, nil)
}

// QueryServersWithProgress is QueryServers with a progress callback. The callback
// may be called from several goroutines at once.
func QueryServersWithProgress(
	multicastGroups mapset.Set[netip.Addr],
	targetPorts mapset.Set[uint16],
	servers map[uuid.UUID]*AnnounceMessage,
	progress Progress,
) {
	sourceToTargetAddrs := sourceToTargetUDPAddrs(
		multicastGroups,
		targetPorts,
	)
	if len(sourceToTargetAddrs) == 0 {
		return
	}
	type connTarget struct {
		conn   *net.UDPConn
		target *net.UDPAddr
	}
	var connTargets []*connTarget
	var conns []*net.UDPConn
	for source, targets := range sourceToTargetAddrs {
		//goland:noinspection GoResourceLeak
		conn, err := net.ListenUDP(
			"udp4",
			source,
		)
		if err != nil {
			continue
		}
		if slices.ContainsFunc(targets, func(addr *net.UDPAddr) bool {
			return addr.IP.IsMulticast()
		}) {
			p := ipv4.NewPacketConn(conn)
			if err = p.SetMulticastLoopback(true); err != nil {
				_ = conn.Close()
				continue
			}
		}
		conns = append(conns, conn)
		for _, target := range targets {
			connTargets = append(connTargets, &connTarget{
				conn:   conn,
				target: target,
			})
		}
	}

	defer func(connections []*net.UDPConn) {
		for _, conn := range connections {
			_ = conn.Close()
		}
	}(conns)

	if len(connTargets) == 0 {
		return
	}

	// Group targets by socket so each socket has exactly one goroutine.
	// Concurrent ReadFromUDP calls on the same socket would race on
	// SetReadDeadline and steal each other's responses.
	type socketGroup struct {
		conn    *net.UDPConn
		targets []*net.UDPAddr
	}
	var socketGroups []*socketGroup
	connIndex := make(map[*net.UDPConn]int)
	for _, ct := range connTargets {
		idx, ok := connIndex[ct.conn]
		if !ok {
			socketGroups = append(socketGroups, &socketGroup{conn: ct.conn})
			idx = len(socketGroups) - 1
			connIndex[ct.conn] = idx
		}
		socketGroups[idx].targets = append(socketGroups[idx].targets, ct.target)
	}

	data := []byte(common.AnnounceHeader)
	var serverLock sync.Mutex

	sendAndReceive := func(packetBuffer *[]byte, conn *net.UDPConn, target *net.UDPAddr, servers map[uuid.UUID]*AnnounceMessage) {
		if _, err := conn.WriteToUDP(data, target); err != nil {
			return
		}
		if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			return
		}
		n, addr, err := conn.ReadFromUDP(*packetBuffer)
		if err != nil {
			return
		}
		if n < len(*packetBuffer) {
			return
		}
		if string((*packetBuffer)[:len(common.AnnounceHeader)]) != common.AnnounceHeader {
			return
		}
		var parsedId uuid.UUID
		parsedId, err = uuid.Parse(string((*packetBuffer)[len(common.AnnounceHeader):]))
		if err != nil {
			return
		}
		func() {
			serverLock.Lock()
			defer serverLock.Unlock()
			var srv *AnnounceMessage
			var ok bool
			if srv, ok = servers[parsedId]; !ok {
				srv = &AnnounceMessage{
					IpAddrs: mapset.NewThreadUnsafeSet[netip.Addr](),
				}
				servers[parsedId] = srv
			}
			srv.IpAddrs.Add(common.NetIPToNetIPAddr(addr.IP))
		}()
	}

	var wg sync.WaitGroup
	for _, sg := range socketGroups {
		wg.Go(func() {
			packetBuffer := make([]byte, len(common.AnnounceHeader)+AnnounceIdLength)
			for round := 1; round <= announceRounds; round++ {
				if round > 1 {
					time.Sleep(time.Second)
				}
				for _, target := range sg.targets {
					sendAndReceive(&packetBuffer, sg.conn, target, servers)
				}
				report(progress, servers, &serverLock, round, announceRounds)
			}
		})
	}
	wg.Wait()
	report(progress, servers, &serverLock, announceRounds, announceRounds)
}

// report forwards one progress tick, if anybody is listening.
//
// found is read under the lock because the map is written by every socket
// goroutine at the same time; reading len() of it unguarded is a data race, and
// the race detector in CI is right about that.
func report(progress Progress, servers map[uuid.UUID]*AnnounceMessage, lock *sync.Mutex, round, rounds int) {
	if progress == nil {
		return
	}
	lock.Lock()
	found := len(servers)
	lock.Unlock()
	progress(round, rounds, found)
}

func calculateBroadcastIPv4(ip net.IP, mask net.IPMask) net.IP {
	broadcast := make(net.IP, len(ip))
	for i, b := range ip {
		broadcast[i] = b | ^mask[i]
	}
	return broadcast
}

func sourceToTargetUDPAddrs(
	multicastGroups mapset.Set[netip.Addr],
	targetPorts mapset.Set[uint16],
) (mapping map[*net.UDPAddr][]*net.UDPAddr) {
	interfaces, err := common.RunningNetworkInterfaces()
	if err != nil {
		return nil
	}
	return buildTargetAddrs(interfaces, multicastGroups, targetPorts)
}

func buildTargetAddrs(
	interfaces map[*net.Interface][]*net.IPNet,
	multicastGroups mapset.Set[netip.Addr],
	targetPorts mapset.Set[uint16],
) (mapping map[*net.UDPAddr][]*net.UDPAddr) {
	mapping = make(map[*net.UDPAddr][]*net.UDPAddr)
	for iff, iffIps := range interfaces {
		for _, n := range iffIps {
			sourceAddr := &net.UDPAddr{
				IP: n.IP,
			}
			mapping[sourceAddr] = make([]*net.UDPAddr, 0)
			if iff.Flags&net.FlagBroadcast != 0 {
				for port := range targetPorts.Iter() {
					mapping[sourceAddr] = append(mapping[sourceAddr], &net.UDPAddr{
						IP:   calculateBroadcastIPv4(sourceAddr.IP.To4(), n.Mask),
						Port: int(port),
					})
				}
			}

			if !multicastGroups.IsEmpty() && iff.Flags&net.FlagMulticast != 0 {
				for multicastGroup := range multicastGroups.Iter() {
					for port := range targetPorts.Iter() {
						mapping[sourceAddr] = append(
							mapping[sourceAddr],
							&net.UDPAddr{
								IP:   multicastGroup.AsSlice(),
								Port: int(port),
							},
						)
					}
				}
			}
		}
	}
	return
}
