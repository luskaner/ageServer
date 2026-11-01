package internal

import launcherCommon "github.com/luskaner/ageServer/launcher-common"

const (
	ErrGameTimeoutStart = iota + launcherCommon.ErrLast
	ErrBattleServerTimeOutStart
	ErrFailedStopServer
	ErrFailedWaitForProcess
	// ErrStopped is a run that ended because it was asked to, from the Exit in
	// the system tray. It is not one of the failures above because nothing failed:
	// the wait was cut short by the user and the teardown that followed ran to
	// the end, which is the opposite of the case the other codes describe. It
	// needs its own code so that the closing notification can tell them apart
	// rather than telling somebody who stopped things on purpose that something
	// went wrong and to go and read the logs.
	ErrStopped
)
