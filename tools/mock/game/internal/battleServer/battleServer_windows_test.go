//go:build windows

package battleServer

import (
	"net"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/uuid"
	"golang.org/x/sys/windows"
)

// Regression: a single malformed datagram on the broadcast port used to abort
// the whole discovery; it must be skipped instead.
func TestListenForBattleServerBroadcastSkipsGarbage(t *testing.T) {
	const gameId = "age2"
	result := make(chan error, 1)
	type out struct {
		msg *battleServer.BroadcastMessage
		ip  net.IP
	}
	results := make(chan out, 1)

	go func() {
		msg, ip, err := listenForBattleServerBroadcast(gameId)
		if err != nil {
			result <- err
			return
		}
		results <- out{msg, ip}
	}()

	// Give the receiver time to bind before flooding.
	time.Sleep(200 * time.Millisecond)

	fd, err := windows.Socket(windows.AF_INET, windows.SOCK_DGRAM, windows.IPPROTO_UDP)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = windows.Closesocket(fd) }()
	addr := windows.SockaddrInet4{
		Port: int(battleServer.BroadcastPort(gameId)),
		Addr: [4]byte{127, 0, 0, 1},
	}

	send := func(payload []byte) error {
		return windows.Sendto(fd, payload, 0, &addr)
	}

	// Garbage first: pre-fix this returned a fatal parse error.
	if err = send([]byte("this is not a broadcast message")); err != nil {
		t.Fatal(err)
	}
	// Then a valid one.
	id := uuid.New()
	idText, _ := id.MarshalText()
	name := battleServer.DefaultName
	payload := make([]byte, 0, 3+36+2+len(name)+6)
	payload = append(payload, 0x21, 0x24, 0x00)
	payload = append(payload, idText...)
	payload = append(payload, byte(len(name)), 0)
	payload = append(payload, name...)
	for _, port := range []uint16{27012, 27112, 27212} {
		payload = append(payload, byte(port), byte(port>>8))
	}
	if err = send(payload); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-result:
		t.Fatalf("garbage packet aborted discovery: %v", err)
	case o := <-results:
		if o.msg == nil || o.msg.Id != id {
			t.Fatalf("unexpected message %+v", o.msg)
		}
		if !o.ip.Equal(net.IPv4(127, 0, 0, 1)) {
			t.Fatalf("sender ip = %v", o.ip)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for broadcast")
	}
}
