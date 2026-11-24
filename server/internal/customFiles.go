package internal

import "time"

// CustomFilesConfiguration is the resolved [CustomFiles] section. The embedded
// ageAPIServer library serves /api/MatchContent now, so these values are handed
// to it at startup instead of configuring a per-game store of the host.
type CustomFilesConfiguration struct {
	Enabled      bool
	Folder       string
	MaxFileSize  int64
	MaxTotalSize int64
	Expiry       time.Duration
}

// CustomFilesConfig is the resolved configuration published at startup.
var CustomFilesConfig CustomFilesConfiguration
