package cmd

import (
	"crypto/x509"
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/hosts"
	"github.com/luskaner/ageServer/common/logger"
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	launcherCommonCmd "github.com/luskaner/ageServer/launcher-common/cmd/config"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	commonUserData "github.com/luskaner/ageServer/launcher-common/userData"
	"github.com/luskaner/ageServer/launcher-config/internal"
	"github.com/spf13/pflag"
)

func removeUserCert() bool {
	commonLogger.Println(commonUi.Step("Removing previously added user certificate, authorize it if needed ..."))
	if _, err := removeUserCertsFn(); err == nil {
		commonLogger.Println(commonUi.Ok("Successfully removed user certificate"))
		return true
	}

	commonLogger.Println(commonUi.Fail("Failed to remove user certificate"))
	return false
}

func restoreMetadata() bool {
	commonLogger.Println(commonUi.Step("Restoring previously backed up metadata"))
	if metadataRestoreFn(path) {
		commonLogger.Println(commonUi.Ok("Successfully restored metadata"))
		return true
	}

	commonLogger.Println(commonUi.Fail("Failed to restore metadata"))
	return false
}

func restoreProfiles() bool {
	commonLogger.Println(commonUi.Step("Restoring previously backed up profiles"))
	if restoreProfilesFn(path, true) {
		commonLogger.Println(commonUi.Ok("Successfully restored profiles"))
		return true
	}

	commonLogger.Println(commonUi.Fail("Failed to restore profiles"))
	return false
}

func restoreGameCert() bool {
	commonLogger.Println(commonUi.Step("Restoring previously added game's certificate store..."))
	if err, _ := newCACertFn(setupValues.GameId, setupValues.GamePath).Restore(); err == nil {
		commonLogger.Println(commonUi.Ok("Successfully restored game's certificate store."))
		return true
	}

	commonLogger.Println(commonUi.Fail("Failed to restore game's certificate store."))
	return false
}

func undoSetUp() {
	if addedUserCert {
		removeUserCert()
	}
	if backedUpMetadata {
		restoreMetadata()
	}
	if backedUpProfiles {
		restoreProfiles()
	}
	if addedGameCert {
		restoreGameCert()
	}
	if setupValues.HostFilePath != "" {
		_ = removeFileFn(setupValues.HostFilePath)
	}
	if setupValues.CertFilePath != "" {
		_ = removeFileFn(setupValues.CertFilePath)
	}
	if setupValues.SigKeyFilePath != "" {
		_ = removeFileFn(setupValues.SigKeyFilePath)
	}
}

var setupValues *launcherCommonCmd.SetupValues

// State
var addedUserCert bool
var backedUpMetadata bool
var backedUpProfiles bool
var addedGameCert bool

func runSetUp(args []string) (err error, exitCode int) {
	// Reset state so a previous (failed or partial) invocation in the same
	// process cannot make undoSetUp revert things this run never did.
	setupValues = nil
	path = nil
	addedUserCert = false
	backedUpMetadata = false
	backedUpProfiles = false
	addedGameCert = false

	var flags *pflag.FlagSet
	setupValues, flags = launcherCommonCmd.SetUpFlagSet()
	if err = flags.Parse(args); err != nil {
		exitCode = common.ErrSyntax
		return
	}

	if setupValues.GameId == "" {
		return errors.New("required flag 'game' not set"), common.ErrSyntax
	}
	common.SetUseInternet(setupValues.CanUseInternet)

	// signal handler
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_, ok := <-sigs
		if ok {
			exitCode = common.ErrSignal
			undoSetUp()
		}
	}()
	if setupValues.LogRoot != "" {
		if initErr := initializeFn(setupValues.LogRoot); initErr != nil {
			commonLogger.Println(commonUi.Fail("Failed to initialize file logging: %s", initErr))
		}
	}
	if setupValues.GameId == game.AoE1 {
		setupValues.Metadata = false
		setupValues.AddCACertData = nil
	} else if setupValues.GameId == game.AoE4 {
		setupValues.AddCACertData = nil
	}
	if setupValues.Metadata || setupValues.Profiles {
		if !supportedGamesContainsFn(setupValues.GameId) {
			commonLogger.Println(commonUi.Fail("Invalid game type"))
			exitCode = launcherCommon.ErrInvalidGame
			return
		}
		var fileInfo os.FileInfo
		if fileInfo, err = statFn(setupValues.DataPath); err != nil || !fileInfo.IsDir() {
			commonLogger.Println(commonUi.Fail("Invalid data path"))
			exitCode = internal.ErrInvalidDataPath
			return
		}
		path = commonUserData.NewPath(setupValues.DataPath, setupValues.GameId)
	}

	var addLocalCertData []byte = nil
	if setupValues.CertFilePath != "" {
		if len(setupValues.AddLocalCertData) == 0 {
			commonLogger.Println(commonUi.Fail("Certificate file path is set but no local certificate data is provided"))
			exitCode = internal.ErrMissingLocalCertData
			return
		}
	} else {
		addLocalCertData = setupValues.AddLocalCertData
	}
	if setupValues.SigKeyFilePath != "" {
		if len(setupValues.AddSigKeyData) == 0 {
			commonLogger.Println(commonUi.Fail("Signing key file path is set but no signing key data is provided"))
			exitCode = internal.ErrMissingSigKeyData
			return
		}
	}
	commonLogger.Println(commonUi.Step("Setting up configuration for %s...", setupValues.GameId))
	if setupValues.AddUserCertData != nil {
		commonLogger.Println(commonUi.Step("Adding user certificate, authorize it if needed..."))
		crt := bytesToCertFn(setupValues.AddUserCertData)
		if crt == nil {
			commonLogger.Println(commonUi.Fail("Failed to parse certificate"))
			exitCode = internal.ErrUserCertAddParse
			undoSetUp()
			return
		}
		if err = addUserCertsFn([]*x509.Certificate{crt}); err == nil {
			commonLogger.Println(commonUi.Ok("Successfully added user certificate"))
			addedUserCert = true
		} else {
			commonLogger.Println(commonUi.Fail("Failed to add user certificate"))
			commonLogger.Println(commonUi.Fault("Error message: %s", err.Error()))
			exitCode = internal.ErrUserCertAdd
			undoSetUp()
			return
		}
	}
	if setupValues.Metadata {
		commonLogger.Println(commonUi.Step("Backing up metadata"))
		if metadataBackupFn(path) {
			commonLogger.Println(commonUi.Ok("Successfully backed up metadata"))
			backedUpMetadata = true
		} else {
			commonLogger.Println(commonUi.Fail("Failed to back up metadata"))
			exitCode = internal.ErrMetadataBackup
			undoSetUp()
			return
		}
	}
	if setupValues.Profiles {
		commonLogger.Println(commonUi.Step("Backing up profiles"))
		if backupProfilesFn(path) {
			commonLogger.Println(commonUi.Ok("Successfully backed up profiles"))
			backedUpProfiles = true
		} else {
			commonLogger.Println(commonUi.Fail("Failed to back up profiles"))
			exitCode = internal.ErrProfilesBackup
			undoSetUp()
			return
		}
	}
	if setupValues.AddCACertData != nil {
		commonLogger.Println(commonUi.Step("Adding certificate to game's store..."))
		if setupValues.GamePath == "" {
			commonLogger.Println(commonUi.Fail("Game path is required to add certificate to game's store"))
			exitCode = internal.ErrGamePathMissing
			undoSetUp()
			return
		}
		crt := bytesToCertFn(setupValues.AddCACertData)
		if crt == nil {
			commonLogger.Println(commonUi.Fail("Failed to parse certificate"))
			exitCode = internal.ErrGameCertAddParse
			undoSetUp()
			return
		}
		gameCert := newCACertFn(setupValues.GameId, setupValues.GamePath)
		if err = gameCert.Backup(); err == nil {
			commonLogger.Println(commonUi.Ok("Successfully backed up game's store."))
			addedGameCert = true
		} else {
			commonLogger.Println(commonUi.Fail("Failed to add certificate to game's store."))
			commonLogger.Println(commonUi.Fault("Error message: %s", err.Error()))
			exitCode = internal.ErrGameCertBackup
			undoSetUp()
			return
		}
		if err = gameCert.Append([]*x509.Certificate{crt}); err == nil {
			commonLogger.Println(commonUi.Ok("Successfully added certificate to game's store."))
		} else {
			commonLogger.Println(commonUi.Fail("Failed to add certificate to game's store."))
			commonLogger.Println(commonUi.Fault("Error message: %s", err.Error()))
			exitCode = internal.ErrGameCertAdd
			undoSetUp()
			return
		}
	}
	var ipToMap net.IP
	if setupValues.HostFilePath == "" {
		if len(setupValues.MapIp) > 0 {
			ipToMap = setupValues.MapIp
		}
	} else if len(setupValues.MapIp) > 0 {
		if ok, addHostsErr := addHostsFn(setupValues.MapIp, setupValues.GameId, setupValues.HostFilePath, hosts.WindowsLineEnding, setupValues.MacOsExclusiveMappings, nil); ok {
			commonLogger.Println(commonUi.Ok("Successfully added host mappings"))
		} else {
			commonLogger.Println(commonUi.Fail("Failed to add host mappings with IP %s to custom hosts file %s", setupValues.MapIp.String(), setupValues.HostFilePath))
			if addHostsErr != nil {
				commonLogger.Println(commonUi.Fault("Error message: %s", addHostsErr))
			} else {
				commonLogger.Println(commonUi.Fault("Error message: unknown (AddHosts returned ok=false with nil error)"))
			}
			exitCode = internal.ErrHostsAdd
			undoSetUp()
			return
		}
	}
	if setupValues.CertFilePath != "" {
		var certFile *os.File
		certFile, err = createFileFn(setupValues.CertFilePath)
		if err == nil {
			err = writeAsPemFn(setupValues.AddLocalCertData, certFile)
			_ = certFile.Close()
		}
		if err != nil {
			commonLogger.Println(commonUi.Fail("Error saving certificate file: %s", err))
			exitCode = internal.ErrUserCertAdd
			undoSetUp()
			return
		}
	}
	if setupValues.SigKeyFilePath != "" {
		var sigKeyFile *os.File
		sigKeyFile, err = createFileFn(setupValues.SigKeyFilePath)
		if err == nil {
			_, err = sigKeyFile.Write(setupValues.AddSigKeyData)
			_ = sigKeyFile.Close()
		}
		if err != nil {
			commonLogger.Println(commonUi.Fail("Error saving signing key file: %s", err))
			exitCode = internal.ErrSigKeyFileAdd
			undoSetUp()
			return
		}
	}
	if addLocalCertData != nil || len(ipToMap) > 0 {
		agentStarted := connectAgentFn() == nil
		if agentStarted {
			commonLogger.Println(commonUi.Step("Communicating with config-admin-agent to add local cert and/or host mappings..."))
		} else {
			str := "Running config-admin to add local cert and/or host mappings"
			if !isAdminFn() {
				str += ", authorize it if needed"
			}
			commonLogger.Println(commonUi.Step("%s", str+"..."))
		}
		err, exitCode = runSetUpAdminFn(setupValues.GameId, setupValues.LogRoot, ipToMap, setupValues.MacOsExclusiveMappings, addLocalCertData, setupValues.CanUseInternet)
		if err == nil && exitCode == common.ErrSuccess {
			if agentStarted {
				commonLogger.Println(commonUi.Ok("Successfully communicated with config-admin-agent"))
			} else {
				commonLogger.Println(commonUi.Ok("Successfully ran config-admin"))
			}
		} else {
			commonLogger.Println(commonUi.Fail("Failed admin setup with IP %s (via agent=%v)", ipToMap.String(), agentStarted))
			if err != nil {
				commonLogger.Println(commonUi.Fault("Received error:"))
				commonLogger.Println(commonUi.Detail("%s", err))
			} else {
				commonLogger.Println(commonUi.Fault("Received error: none (exit code only)"))
			}
			if exitCode != common.ErrSuccess {
				commonLogger.Println(commonUi.Fault("Received exit code:"))
				commonLogger.Println(commonUi.Detail("%d", exitCode))
			}
			if !agentStarted {
				commonLogger.Println(commonUi.Detail("Check the config-admin_setup_hosts log file for the underlying config-admin output."))
			}
			exitCode = internal.ErrAdminSetup
			if agentStarted {
				commonLogger.Println(commonUi.Fail("Failed to communicate with config-admin-agent. Communicating with it to shutdown..."))
				if setupValues.AgentEndOnError {
					_ = stopAgentIfNeededFn()
				}
				undoSetUp()
			}
		}
	}
	return
}
