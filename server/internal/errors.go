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
	ErrBattleServerReplays
)
