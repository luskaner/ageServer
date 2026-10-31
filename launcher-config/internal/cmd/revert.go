package cmd

import (
	"crypto/x509"
	"errors"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/logger"
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	launcherCommonCmd "github.com/luskaner/ageServer/launcher-common/cmd/config"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	commonUserData "github.com/luskaner/ageServer/launcher-common/userData"
	"github.com/luskaner/ageServer/launcher-config/internal"
	"github.com/spf13/pflag"
)

func addUserCerts(removedUserCerts []*x509.Certificate) bool {
	commonLogger.Println(commonUi.Step("Adding previously removed user certificate"))
	if err := addUserCertsFn(removedUserCerts); err == nil {
		commonLogger.Println(commonUi.Ok("Successfully added user certificate"))
		return true
	}
	commonLogger.Println(commonUi.Fail("Failed to add user certificate"))
	return false
}

func backupMetadata() bool {
	commonLogger.Println(commonUi.Step("Backing up previously restored metadata"))
	if metadataBackupFn(path) {
		commonLogger.Println(commonUi.Ok("Successfully backed up metadata"))
		return true
	}
	commonLogger.Println(commonUi.Fail("Failed to back up metadata"))
	return false
}

func backupProfiles() bool {
	commonLogger.Println(commonUi.Step("Backing up previously restored profiles"))
	if backupProfilesFn(path) {
		commonLogger.Println(commonUi.Ok("Successfully backed up profiles"))
		return true
	}
	commonLogger.Println(commonUi.Fail("Failed to back up profiles"))
	return false
}

func addCaCerts(removedCaCerts []*x509.Certificate) bool {
	commonLogger.Println(commonUi.Step("Restoring previously added game's certificate store..."))
	if err := newCACertFn(revertValues.GameId, revertValues.GamePath).Append(removedCaCerts); err == nil {
		commonLogger.Println(commonUi.Ok("Successfully restored game's certificate store."))
		return true
	}
	commonLogger.Println(commonUi.Fail("Failed to restore game's certificate store."))
	return false
}

func undoRevert() {
	if !revertValues.RemoveAll {
		if removedCaCerts != nil {
			addCaCerts(removedCaCerts)
		}
		if removedUserCerts != nil {
			addUserCerts(removedUserCerts)
		}
		if restoredMetadata {
			backupMetadata()
		}
		if restoredProfiles {
			backupProfiles()
		}
	}
}

var revertValues *launcherCommonCmd.RevertValues

// State
var removedUserCerts []*x509.Certificate
var removedCaCerts []*x509.Certificate
var restoredMetadata bool
var restoredProfiles bool

func runRevert(args []string) (err error, exitCode int) {
	// Reset state so a previous invocation in the same process cannot leak
	// into this one.
	revertValues = nil
	removedUserCerts = nil
	removedCaCerts = nil
	restoredMetadata = false
	restoredProfiles = false

	// config-admin-agent is an elevated daemon that must never outlive this
	// command. Whether there is one to stop is decided here, once, and the
	// shutdown runs from a defer: it used to be the last statement of the happy
	// path, so every early return below (invalid game or data path, user cert /
	// metadata / profile / game cert failures, ErrAdminRevert, signals) left an
	// elevated process running that only the user could kill.
	//
	// The gate is deliberately ConfigAdminAgentRunning and not
	// RevertRequiresAdminElevationValues: whether the revert needs elevation
	// says nothing about whether an agent is alive, so keying the shutdown off
	// it skipped the stop entirely for custom hosts/cert files.
	agentPresent := configAdminAgentRunningFn()
	defer func() {
		if !agentPresent {
			return
		}
		if !stopAgentIfNeededFn() && exitCode == common.ErrSuccess {
			exitCode = internal.ErrRevertStopAgent
		}
	}()

	var flags *pflag.FlagSet
	revertValues, flags = launcherCommonCmd.RevertFlagSet()
	if err = flags.Parse(args); err != nil {
		exitCode = common.ErrSyntax
		return
	}
	if revertValues.GameId == "" {
		return errors.New("required flag 'game' not set"), common.ErrSyntax
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_, ok := <-sigs
		if ok {
			undoRevert()
			exitCode = common.ErrSignal
		}
	}()
	if revertValues.LogRoot != "" {
		if initErr := initializeFn(revertValues.LogRoot); initErr != nil {
			commonLogger.Println(commonUi.Fail("Failed to initialize file logging: %s", initErr))
		}
	}
	reverseFailed := true
	if revertValues.RemoveAll {
		revertValues.IPs = true
		revertValues.Certs = true
		if runtime.GOOS != "linux" {
			revertValues.RemoveUserCert = true
		}
		revertValues.Metadata = true
		revertValues.Profiles = true
		revertValues.RestoreCAStoreCert = true
		reverseFailed = false
	}
	if revertValues.GameId == game.AoE1 {
		revertValues.Metadata = false
		revertValues.RestoreCAStoreCert = false
	} else if revertValues.GameId == game.AoE4 {
		revertValues.RestoreCAStoreCert = false
	}
	if revertValues.Metadata || revertValues.Profiles {
		if !supportedGamesContainsFn(revertValues.GameId) {
			commonLogger.Println(commonUi.Fail("Invalid game type"))
			exitCode = launcherCommon.ErrInvalidGame
			undoRevert()
			return
		}
		var fileInfo os.FileInfo
		if fileInfo, err = statFn(revertValues.DataPath); err != nil || !fileInfo.IsDir() {
			commonLogger.Println(commonUi.Fail("Invalid data path"))
			exitCode = internal.ErrInvalidDataPath
			undoRevert()
			return
		}
		path = commonUserData.NewPath(revertValues.DataPath, revertValues.GameId)
	}
	commonLogger.Println(commonUi.Step("Reverting configuration for %s...", revertValues.GameId))
	if revertValues.RemoveUserCert {
		commonLogger.Println(commonUi.Step("Removing user certificates, authorize it if needed..."))
		if removedUserCerts, _ = removeUserCertsFn(); removedUserCerts != nil {
			commonLogger.Println(commonUi.Ok("Successfully removed user certificates"))
		} else {
			commonLogger.Println(commonUi.Fail("Failed to remove user certificates"))
			exitCode = internal.ErrUserCertRemove
			undoRevert()
			return
		}
	}
	if revertValues.Metadata {
		commonLogger.Println(commonUi.Step("Restoring metadata"))
		if metadataRestoreFn(path) {
			commonLogger.Println(commonUi.Ok("Successfully restored metadata"))
			restoredMetadata = true
		} else {
			commonLogger.Println(commonUi.Fail("Failed to restore metadata"))
			exitCode = internal.ErrMetadataRestore
			undoRevert()
			return
		}
	}
	if revertValues.Profiles {
		commonLogger.Println(commonUi.Step("Restoring profiles"))
		if restoreProfilesFn(path, reverseFailed) {
			commonLogger.Println(commonUi.Ok("Successfully restored profiles"))
			restoredProfiles = true
		} else {
			commonLogger.Println(commonUi.Fail("Failed to restore profiles"))
			exitCode = internal.ErrProfilesRestore
			undoRevert()
			return
		}
	}
	if revertValues.RestoreCAStoreCert {
		commonLogger.Println(commonUi.Step("Restoring original certificate game's store..."))
		if revertValues.GamePath == "" {
			commonLogger.Println(commonUi.Fail("Game path is required to restore the original game's store"))
			exitCode = internal.ErrGamePathMissing
			undoRevert()
			return
		}
		cert := newCACertFn(revertValues.GameId, revertValues.GamePath)
		if err, removedCaCerts = cert.Restore(); err == nil {
			commonLogger.Println(commonUi.Ok("Successfully restored original game's store."))
		} else {
			commonLogger.Println(commonUi.Fail("Failed to restore original game's store."))
			commonLogger.Println(commonUi.Fault("Received error:"))
			commonLogger.Println(commonUi.Detail("%s", err))
			exitCode = internal.ErrGameCertRestore
			undoRevert()
			return
		}
	}
	var agentConnected *bool
	if launcherCommon.RevertRequiresAdminElevationValues(revertValues) {
		agentConnected = new(connectAgentFn() == nil)
		if *agentConnected {
			// The agent is live and answering, so make sure the defer stops it.
			agentPresent = true
			commonLogger.Println(commonUi.Step("Communicating with config-admin-agent to remove local cert and/or host mappings..."))
		} else {
			str := "Running config-admin to remove local cert and/or host mappings"
			if !isAdminFn() {
				str += ", authorize it if needed"
			}
			commonLogger.Println(commonUi.Step("%s", str+"..."))
		}
		err, exitCode = runRevertAdminFn(revertValues.LogRoot, revertValues.IPs, revertValues.Certs, !revertValues.RemoveAll)
		if err == nil && exitCode == common.ErrSuccess {
			if *agentConnected {
				commonLogger.Println(commonUi.Ok("Successfully communicated with config-admin-agent"))
			} else {
				commonLogger.Println(commonUi.Ok("Successfully ran config-admin"))
			}
		} else {
			if err != nil {
				commonLogger.Println(commonUi.Fault("Received error:"))
				commonLogger.Println(commonUi.Detail("%s", err))
			}
			if exitCode != common.ErrSuccess {
				commonLogger.Println(commonUi.Fault("Received exit code:"))
				commonLogger.Println(commonUi.Detail("%d", exitCode))
			}
			exitCode = internal.ErrAdminRevert
			if *agentConnected {
				commonLogger.Println(commonUi.Fail("Failed to communicate with config-admin-agent"))
			} else {
				commonLogger.Println(commonUi.Fail("Failed to run config-admin"))
			}
			undoRevert()
			return
		}
	}
	// Ignore previous error if we don't failfast
	if revertValues.RemoveAll {
		exitCode = common.ErrSuccess
	}
	if exitCode == common.ErrSuccess && revertValues.HostFilePath != "" {
		_ = removeFileFn(revertValues.HostFilePath)
	}
	if exitCode == common.ErrSuccess && revertValues.CertFilePath != "" {
		_ = removeFileFn(revertValues.CertFilePath)
	}
	if exitCode == common.ErrSuccess && revertValues.SigKeyFilePath != "" {
		_ = removeFileFn(revertValues.SigKeyFilePath)
	}
	return
}
