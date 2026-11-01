package battleServer

import (
	"errors"
	"fmt"
	"log"
	"net"
	"slices"
	"strconv"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/process"
	"golang.org/x/sys/windows"
)

func listenForBattleServerBroadcast(gameId string) (*battleServer.BroadcastMessage, net.IP, error) {
	fd, err := windows.Socket(windows.AF_INET, windows.SOCK_DGRAM, windows.IPPROTO_UDP)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create socket: %w", err)
	}
	defer func() {
		_ = windows.Closesocket(fd)
	}()
	if err = windows.SetsockoptInt(fd, windows.SOL_SOCKET, windows.SO_REUSEADDR, 1); err != nil {
		return nil, nil, fmt.Errorf("failed to set SO_REUSEADDR: %w", err)
	}
	if err = windows.SetsockoptInt(fd, windows.SOL_SOCKET, windows.SO_RCVTIMEO, 15000); err != nil {
		return nil, nil, fmt.Errorf("failed to set read timeout (SO_RCVTIMEO): %w", err)
	}
	port := int(battleServer.BroadcastPort(gameId))
	addr := windows.SockaddrInet4{
		Port: port,
		Addr: [4]byte(net.IPv4zero.To4()),
	}
	if err = windows.Bind(fd, &addr); err != nil {
		return nil, nil, fmt.Errorf("failed to bind socket on port %d: %w", port, err)
	}
	localIPs := common.ResolveUnspecifiedIps()
	buffer := make([]byte, 65535)
	var msg *battleServer.BroadcastMessage
	var ip net.IP
	log.Println("Listening on battle server broadcast messages")

	for {
		n, from, readErr := windows.Recvfrom(fd, buffer, 0)
		if readErr != nil {
			if errno, ok := errors.AsType[windows.Errno](readErr); ok && (errors.Is(errno, windows.WSAETIMEDOUT) || errors.Is(errno, windows.WSAEINTR)) {
				break
			}
			return nil, nil, readErr
		}
		var senderIP net.IP
		if sa, ok := from.(*windows.SockaddrInet4); ok {
			senderIP = sa.Addr[:]
		} else {
			continue
		}

		if !slices.Contains(localIPs, senderIP.String()) {
			continue
		}

		if tmpMsg, parseErr := battleServer.ParseBroadcastMessage(buffer[:n], n); parseErr == nil {
			if tmpMsg.Name == battleServer.DefaultName {
				msg = tmpMsg
				ip = senderIP
				break
			}
		} else {
			// Stray or malformed datagrams on the broadcast port must be
			// skipped, not fatal: other traffic may share the port.
			log.Printf("Skipping invalid broadcast message: %v", parseErr)
			continue
		}
	}

	return msg, ip, nil
}

func StartAndCheck(gameId string) (err error) {
	var path string
	var ok bool
	if ok, path = battleServer.ResolvePath(gameId); !ok {
		return fmt.Errorf("could not find battle server executable")
	}
	// TODO: Add other arguments when implemented
	options := exec.Options{
		File:           path,
		UseWorkingPath: true,
		Pid:            true,
	}
	if result := options.Exec(); result.Success() {
		log.Println("Started battle server with PID:", result.Pid)
		if proc, err := process.FindProcess(int(result.Pid)); err == nil {
			defer func() {
				_ = process.KillProc(proc)
			}()
		}
		var msg *battleServer.BroadcastMessage
		var ip net.IP
		msg, ip, err = listenForBattleServerBroadcast(gameId)
		if err != nil {
			return err
		}
		if msg == nil {
			return fmt.Errorf("did not receive an appropriate battle server broadcast message within the deadline")
		}
		ports := []uint16{msg.PublicPort, msg.WebsocketPort, msg.OutOfBandPort}
		for _, port := range ports {
			if port == 0 {
				continue
			}
			target := net.JoinHostPort(ip.String(), strconv.Itoa(int(port)))
			testConn, portErr := net.DialTimeout("tcp4", target, 100*time.Millisecond)
			if portErr != nil {
				return fmt.Errorf("failed to connect to battle server on port %d: %w", port, portErr)
			}
			_ = testConn.Close()
		}
	} else {
		return fmt.Errorf("could not start battle server executable")
	}
	return nil
}
