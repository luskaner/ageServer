package internal

import launcherCommon "github.com/luskaner/ageServer/launcher-common"

const (
	ErrListen = iota + launcherCommon.ErrLast
	ErrDecode
	ErrNonExistingAction
	ErrConnectionClosing
	// ErrCertAlreadyAdded and ErrIpsAlreadyMapped stopped being returned when
	// a session became allowed to add certificates and hosts repeatedly (an
	// API pass and the server's own pass share one agent). They stay here so
	// removing them does not renumber the exit codes that follow, like
	// launcher-common's ErrInvalidDialog.
	ErrCertAlreadyAdded
	ErrIpsAlreadyMapped
	ErrCertInvalid
	ErrFlushCache
	ErrGameNotSupported
	ErrHostSetConflict
)
