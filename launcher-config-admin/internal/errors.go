package internal

import (
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
)

const (
	ErrLocalCertRemove = iota + launcherCommon.ErrLast
	ErrIpMapRemove
	ErrIpMapRemoveRevert
	ErrLocalCertAdd
	ErrLocalCertAddParse
	ErrIpMapAdd
	ErrIpMapAddRevert
	ErrFlushCache
	ErrFlushCacheDNS
	ErrFlushCacheCerts
)
