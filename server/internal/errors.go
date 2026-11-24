package internal

import (
	"github.com/luskaner/ageServer/common"
)

const (
	ErrCertDirectory = iota + common.ErrLast
	ErrResolveHost
	ErrCreateLogFile
	ErrStartServer
	ErrMulticastGroup
	ErrGames
	ErrGame
	ErrAnnounce
	ErrInvalidId
	ErrInvalidAuthentication
	ErrSigningKey
	ErrCustomFiles
	ErrEmbeddedBattleServer
	ErrReplays
	ErrMods
	// ErrApiDisabledWithMods is the code for a configuration that turns the API
	// tree off while asking for the mods API that lives in it.
	ErrApiDisabledWithMods
	// ErrSafe is the code for an invalid [Safe] configuration.
	ErrSafe
)
