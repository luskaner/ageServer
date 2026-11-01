package launcher

import (
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
)

const (
	ErrInvalidCanTrustCertificate = iota + launcherCommon.ErrLast
	ErrInvalidCanBroadcastBattleServer
	ErrInvalidServerStart
	ErrInvalidServerStop
	ErrInvalidServerHost
	ErrGameAlreadyRunning
	ErrGameLauncherNotFound
	ErrGameLauncherStart
	ErrServerExecutable
	ErrServerConnectSecure
	ErrServerUnreachable
	ErrServerCertMissingExpired
	ErrServerCertDirectory
	ErrServerCertCreate
	ErrServerStart
	ErrConfigIpMap
	ErrGameUnsupportedLauncherCombo
	ErrConfigIpMapAdd
	ErrConfigCertAdd
	ErrConfigCACertAdd
	ErrConfigCert
	ErrReadCert
	ErrTrustCert
	ErrMetadataProfilesSetup
	ErrAgentStart
	ErrInvalidClientPath
	ErrInvalidServerArgs
	ErrInvalidServerPath
	ErrInvalidClientArgs
	ErrInvalidSetupCommand
	ErrInvalidRevertCommand
	ErrInvalidIsolationPath
	ErrSetupCommand
	ErrAnnouncementMulticastGroup
	ErrCertMismatch
	ErrInvalidServerBattleServerManagerRun
	ErrInvalidServerBattleServerManagerArgs
	ErrBattleServerManagerRun
	ErrInvalidIsolateMetadata
	ErrInvalidIsolateProfiles
	ErrRequiredIsolation
	ErrGameConfigParse
	ErrFlushCache
	ErrInvalidDialog
	ErrServerStartCanceled
	// ErrCanceled is the code for a run that was asked to stop before it
	// finished.
	//
	// It is distinct from success because the user asked for something and did
	// not get it: nothing was set up, or something was set up and put back, and
	// either way the game did not run. A caller that only cares whether the
	// machine is clean can ignore it; a caller showing a result should not tell
	// someone a cancelled launch worked.
	ErrCanceled
	// ErrReadSigKey is the code for a run that needed the server's public
	// signing key and could not get a valid one from it.
	ErrReadSigKey
	// ErrConfigSigKeyAdd is the code for a run that got the signing key but
	// could not leave it in a file for the client to read.
	ErrConfigSigKeyAdd
)
