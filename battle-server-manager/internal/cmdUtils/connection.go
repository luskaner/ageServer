package cmdUtils

import (
	"net"
	"runtime"
	"time"

	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/cmdlog"
)

var waitInitTimeout = 10 * time.Second

// waitPortDialTimeout is how long each readiness probe may wait for a port.
//
// Config.Validate's own default is 100ms, which is right when classifying stored
// configs as stale, where a closed port refuses instantly and you want the answer
// quickly. It is wrong for deciding whether a battle server we just spawned has
// finished binding: a loaded machine can take longer than 100ms to complete the
// handshake, the readiness loop gives up after waitInitTimeout and then kills the
// process it just started. A refused connection still returns immediately, so a
// generous timeout costs nothing while the port is genuinely closed.
const waitPortDialTimeout = 1 * time.Second

func waitDial(network, address string, _ time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, waitPortDialTimeout)
}

func WaitForBattleServerInit(config battleServer.Config) (ok bool) {
	// Wait for initialization
	t := waitInitTimeout
	if runtime.GOOS != "windows" {
		t *= 3
	}
	timeout := time.After(t)
	cmdlog.Step("Waiting up to %s for the initialization to complete...", t)
loop:
	for {
		select {
		case <-timeout:
			break loop
		default:
			if ok = config.ValidateWith(false, nil, waitDial); ok {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return
}
