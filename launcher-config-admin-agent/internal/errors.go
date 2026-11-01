package internal

import launcherCommon "github.com/luskaner/ageServer/launcher-common"

const (
	ErrListen = iota + launcherCommon.ErrLast
	ErrDecode
	ErrNonExistingAction
	ErrConnectionClosing
	ErrCertAlreadyAdded
	ErrIpsAlreadyMapped
	ErrCertInvalid
	ErrFlushCache
	ErrGameNotSupported
)
