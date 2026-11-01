package watch

import (
	"context"

	"github.com/luskaner/ageLANServer/battle-server-broadcast"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/launcher-agent/internal"
)

// rebroadcastBattleServer starts mirroring the battle server's announcements
// onto the local network, once the battle server is actually up.
//
// The wait here is a second wait inside a run that is already waiting for a game,
// and it took the same shape as the first one did: one minute, no way to stop it
// early. It now takes the run's context like every other wait, so an agent that
// has been asked to stop does not sit here for the rest of the timeout before
// starting the teardown the user is waiting for.
func rebroadcastBattleServer(ctx context.Context, exitCode *ExitCode, port int) {
	mostPriority, restInterfaces, err := battle_server_broadcast.RetrieveBsInterfaceAddresses()
	if err == nil && mostPriority != nil && len(restInterfaces) > 0 {
		if len(waitUntilAnyProcessExist(ctx, []string{battleServer.Executable}, processWaitInterval, oneMinuteWaitTimeout)) > 0 {
			go func() {
				_ = battle_server_broadcast.CloneAnnouncements(mostPriority, restInterfaces, port)
			}()
		} else if !stopped(ctx) {
			exitCode.SetIfSuccess(internal.ErrBattleServerTimeOutStart)
		}
	}
}
