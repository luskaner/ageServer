package launcher_common

import "github.com/luskaner/ageServer/common"

const (
	ErrNotAdmin = iota + common.ErrLast
	ErrInvalidGame
	// ErrLast Only used as a marker to where to start
	ErrLast
)
