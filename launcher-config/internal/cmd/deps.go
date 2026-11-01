package cmd

import (
	"crypto/x509"
	"net"
	"os"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/hosts"
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	commonUserData "github.com/luskaner/ageServer/launcher-common/userData"
	"github.com/luskaner/ageServer/launcher-config/internal"
	"github.com/luskaner/ageServer/launcher-config/internal/admin"
	"github.com/luskaner/ageServer/launcher-config/internal/cmd/wrapper"
	"github.com/luskaner/ageServer/launcher-config/internal/userData"
)

type caCertifier interface {
	Backup() error
	Restore() (error, []*x509.Certificate)
	Append(certs []*x509.Certificate) error
}

var (
	isAdminFn                 = executor.IsAdmin
	connectAgentFn            = admin.ConnectAgentIfNeeded
	connectAgentRetriesFn     = admin.ConnectAgentIfNeededWithRetries
	runSetUpAdminFn           = admin.RunSetUp
	runRevertAdminFn          = admin.RunRevert
	runFlushCacheAdminFn      = admin.RunFlushCache
	startAgentFn              = admin.StartAgent
	stopAgentIfNeededFn       = admin.StopAgentIfNeeded
	configAdminAgentRunningFn = func() bool { return launcherCommon.ConfigAdminAgentRunning(false) }
	removeUserCertsFn         = wrapper.RemoveUserCerts
	addUserCertsFn            = wrapper.AddUserCerts
	newCACertFn               func(gameId string, gamePath string) caCertifier
	initializeFn              = internal.Initialize
	metadataFn                func(path *commonUserData.Path) userData.Data
	metadataBackupFn          func(path *commonUserData.Path) bool
	metadataRestoreFn         func(path *commonUserData.Path) bool
	backupProfilesFn          func(path *commonUserData.Path) bool
	restoreProfilesFn         func(path *commonUserData.Path, reverseFailed bool) bool
	addHostsFn                func(ip net.IP, gameId string, hostFilePath string, lineEnding string, withMacOsExclusive bool, flushFn func() *exec.Result) (bool, error)
	bytesToCertFn             = common.BytesToCertificate
	writeAsPemFn              = common.WriteAsPem
	createFileFn              = os.Create
	removeFileFn              = os.Remove
	statFn                    = os.Stat
	supportedGamesContainsFn  = func(gameId string) bool { return game.SupportedGames.ContainsOne(gameId) }
)

func init() {
	newCACertFn = func(gameId string, gamePath string) caCertifier {
		return internal.NewCACert(gameId, gamePath)
	}
	metadataFn = func(path *commonUserData.Path) userData.Data {
		return userData.Metadata(path)
	}
	metadataBackupFn = func(path *commonUserData.Path) bool {
		return userData.Metadata(path).Backup()
	}
	metadataRestoreFn = func(path *commonUserData.Path) bool {
		return userData.Metadata(path).Restore()
	}
	backupProfilesFn = func(path *commonUserData.Path) bool {
		return userData.BackupProfiles(path)
	}
	restoreProfilesFn = func(path *commonUserData.Path, reverseFailed bool) bool {
		return userData.RestoreProfiles(path, reverseFailed)
	}
	addHostsFn = func(ip net.IP, gameId string, hostFilePath string, lineEnding string, withMacOsExclusive bool, flushFn func() *exec.Result) (bool, error) {
		return hosts.AddHosts(ip, gameId, hostFilePath, lineEnding, withMacOsExclusive, flushFn)
	}
}
