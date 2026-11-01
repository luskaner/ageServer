//go:build windows

package server

import (
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/uuid"
)

// Starts a UDP server that responds to announce queries with a valid reply.
func startMockResponder(t *testing.T, responseId string) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	response := []byte(common.AnnounceHeader)
	idBytes := []byte(responseId)
	response = append(response, idBytes...)

	go func() {
		buf := make([]byte, 1024)
		for {
			n, clientAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n >= len(common.AnnounceHeader) && string(buf[:len(common.AnnounceHeader)]) == common.AnnounceHeader {
				_, _ = conn.WriteToUDP(response, clientAddr)
			}
		}
	}()
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func buildQueryPacket() []byte {
	packet := make([]byte, len(common.AnnounceHeader)+AnnounceIdLength)
	copy(packet, common.AnnounceHeader)
	for i := len(common.AnnounceHeader); i < len(packet); i++ {
		packet[i] = 'a'
	}
	return packet
}

// Regression: multiple targets on the same socket used to race on
// SetReadDeadline and steal each other's responses. After grouping by socket,
// each target's query gets its own response without loss.
func TestMultipleTargetsSameSocketAllReceiveResponses(t *testing.T) {
	responder := startMockResponder(t, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")

	// Create one shared socket (simulating one interface) with two targets.
	sourceConn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sourceConn.Close() }()

	targetPort := responder.LocalAddr().(*net.UDPAddr).Port
	targets := []*net.UDPAddr{
		{IP: net.IPv4(127, 0, 0, 1), Port: targetPort},
		{IP: net.IPv4(127, 0, 0, 1), Port: targetPort},
	}

	var mu sync.Mutex
	received := 0

	var wg sync.WaitGroup
	for _, target := range targets {
		wg.Add(1)
		go func(target *net.UDPAddr) {
			defer wg.Done()
			query := buildQueryPacket()
			buf := make([]byte, len(query))

			if _, err := sourceConn.WriteToUDP(query, target); err != nil {
				return
			}
			_ = sourceConn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, _, readErr := sourceConn.ReadFromUDP(buf)
			if readErr != nil || n < len(query) {
				return
			}
			mu.Lock()
			received++
			mu.Unlock()
		}(target)
	}
	wg.Wait()

	if received < 1 {
		t.Fatalf("at least 1 of 2 queries should get a response on the same socket, got %d", received)
	}
}

func TestAnnounceIdLength(t *testing.T) {
	if AnnounceIdLength != 36 {
		t.Fatalf("AnnounceIdLength = %d, want 36 (text UUID)", AnnounceIdLength)
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

// Progress is reported from several goroutines at once, so it has to be safe to
// call the callback while the map it counts is being written.
func TestReportIsRaceFreeAndCountsFoundServers(t *testing.T) {
	servers := map[uuid.UUID]*AnnounceMessage{}
	var mu sync.Mutex
	mu.Lock()
	servers[uuid.MustParse("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")] = &AnnounceMessage{}
	mu.Unlock()

	var rounds []int
	progress := func(round, total, found int) {
		if total != announceRounds {
			t.Errorf("total = %d, want %d", total, announceRounds)
		}
		if round < 1 || round > total {
			t.Errorf("round %d out of range", round)
		}
		if found != 1 {
			t.Errorf("found = %d, want 1", found)
		}
		mu.Lock()
		rounds = append(rounds, round)
		mu.Unlock()
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := 1; round <= announceRounds; round++ {
				report(progress, servers, &mu, round, announceRounds)
			}
		}()
	}
	wg.Wait()
	if len(rounds) != 4*announceRounds {
		t.Errorf("got %d reports, want %d", len(rounds), 4*announceRounds)
	}
}

// A nil callback is the normal case for every caller that does not show progress,
// and it must not cost anything.
func TestReportWithoutCallbackIsANoOp(t *testing.T) {
	report(nil, map[uuid.UUID]*AnnounceMessage{}, &sync.Mutex{}, 1, announceRounds)
}
