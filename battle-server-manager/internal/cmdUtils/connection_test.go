package cmdUtils

import (
	"net"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common/battleServer"
)

// Regression: the readiness probe inherited Config.Validate's 100ms default,
// which is sized for classifying stored configs as stale, not for deciding
// whether a battle server this process just spawned has finished binding its
// ports. On a loaded machine the handshake can take longer than that, the
// waitInitTimeout loop gives up and the caller kills the process it just
// started.
func TestWaitDialIsMorePatientThanTheDefault(t *testing.T) {
	if waitPortDialTimeout <= 100*time.Millisecond {
		t.Errorf("waitPortDialTimeout = %v, must be longer than Validate's 100ms default", waitPortDialTimeout)
	}
	// It still has to fit inside the overall budget with room for several
	// attempts across the ports being probed.
	if waitPortDialTimeout*3 >= waitInitTimeout {
		t.Errorf("probing 3 ports at %v each would consume the whole %v budget in one pass",
			waitPortDialTimeout, waitInitTimeout)
	}
}

// A port that is not listening must still be reported as invalid promptly, so a
// generous timeout does not turn the readiness loop into a stall.
func TestWaitDialFailsFastOnClosedPort(t *testing.T) {
	// Bind and immediately release, so we know the port was free.
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	start := time.Now()
	if _, dialErr := waitDial("tcp4", addr, 0); dialErr == nil {
		t.Fatal("expected a closed port to refuse")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("closed port took %v to refuse, the timeout is not a stall", elapsed)
	}
}

// The readiness loop must use the patient dialer, not Validate's default.
func TestWaitForBattleServerInitUsesPatientDialer(t *testing.T) {
	cfg := battleServer.Config{
		Region:        "eu",
		IPv4:          "127.0.0.1",
		BsPort:        1,
		WebSocketPort: 2,
	}
	origInit := waitInitTimeout
	waitInitTimeout = 150 * time.Millisecond
	defer func() { waitInitTimeout = origInit }()

	// Nothing is listening on ports 1 and 2, so this must give up quickly and
	// report failure. It exercises the real loop, including waitDial.
	start := time.Now()
	if ok := WaitForBattleServerInit(cfg); ok {
		t.Fatal("expected failure, nothing is listening")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("readiness loop took %v for a clearly dead server", elapsed)
	}
}
