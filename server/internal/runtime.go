package internal

import (
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/uuid"
)

var Id uuid.UUID
var AnnounceMessageData map[string]common.AnnounceMessageData002
var GeneratePlatformUserId bool

// SignConfigRuntime holds the signing configuration at runtime.
var SignConfigRuntime struct {
	// Enabled makes the server re-sign the catalogues it serves instead of
	// replaying the signatures captured alongside them in resources/responses.
	// Off by default: the captured ones are what an unmodified client accepts.
	Enabled bool

	// PlatformUserIdExceptions is a list of platform user IDs for which signing
	// is disabled even when Enabled is true. This allows selective signing:
	// when Enabled is true, all users get signed responses except those listed here.
	PlatformUserIdExceptions []string
}

var CanUseInternet bool
var Authentication string

// CustomFiles holds the resolved /api/MatchContent/ configuration. Set once at
// startup, before any game is created, so both the model and the HTTP handlers
// see the same values without dragging the whole config around.
var CustomFilesEnabled bool
var CustomFilesFolder string
var CustomFilesMaxFileSize int64
var CustomFilesMaxTotalSize int64
var CustomFilesExpiry time.Duration

// BattleServerReplays holds the resolved battle server replay recording configuration.
// Set once at startup.
var BattleServerReplaysEnabled bool
var BattleServerReplaysFolder string
var BattleServerReplaysMaxFileSize int64
var BattleServerReplaysMaxTotalSize int64
var BattleServerReplaysExpiry time.Duration

type AnnounceMessageDataLatest = common.AnnounceMessageData002
