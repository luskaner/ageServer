package ip

import (
	"net"
	"testing"
	"time"
)

// Regression: ListenQueryConnections' goroutine used to `continue` on every
// ReadFromUDP error, hot-spinning at 100% CPU when the socket was closed.
// The fix checks for net.ErrClosed and returns instead.
func TestListenQueryConnectionsExitsOnClosedSocket(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Skip(err)
	}

	done := make(chan struct{})
	go func() {
		ListenQueryConnections([]*net.UDPConn{conn})
		close(done)
	}()

	// Close the socket: the goroutine should detect ErrClosed and exit
	// rather than spinning forever.
	_ = conn.Close()

	select {
	case <-done:
		// Goroutine exited correctly after detecting closed socket.
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not exit after socket was closed (hot-spin)")
	}
}
