package internal

import (
	"github.com/luskaner/ageServer/common"
)

const (
	ErrCertDirectory = iota + common.ErrLast
	ErrCertCreate
	ErrCertCreateExisting
)
