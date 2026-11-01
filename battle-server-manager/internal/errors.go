package internal

import "github.com/luskaner/ageServer/common"

const (
	ErrGames = iota + common.ErrLast
	ErrReadConfig
	ErrAlreadyRunning
	ErrAlreadyExists
	ErrResolveHost
	ErrInvalidHost
	ErrBsPortInUse
	ErrWsPortInUse
	ErrOobPortInUse
	ErrGenPorts
	ErrResolveSSLFiles
	ErrResolvePath
	ErrParseArgs
	ErrStartBattleServer
	ErrInitBattleServer
	ErrConfigWrite
)
