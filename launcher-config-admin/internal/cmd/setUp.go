package cmd

import (
	"crypto/x509"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/luskaner/ageServer/common"
	launcherCommonHosts "github.com/luskaner/ageServer/common/hosts"
	"github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/cmd/config/admin"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher-config-admin/internal"
	"github.com/luskaner/ageServer/launcher-config-admin/internal/hosts"
)

func untrustCertificate() bool {
	commonLogger.Println(commonUi.Step("Removing previously added local certificate"))
	if _, err := untrustCertsFn(false); err == nil {
		commonLogger.Println(commonUi.Ok("Successfully removed local certificate"))
		return true
	}
	commonLogger.Println(commonUi.Fail("Failed to remove local certificate"))
	return false
}

func logHostsDiagnostics() {
	hostsPath := launcherCommonHosts.Path()
	commonLogger.Println(commonUi.Detail("Hosts file path: %q", hostsPath))
	if info, statErr := os.Stat(hostsPath); statErr != nil {
		commonLogger.Println(commonUi.Detail("Hosts file stat failed: %v", statErr))
	} else {
		commonLogger.Println(commonUi.Detail("Hosts file info: size=%d mode=%v isDir=%v", info.Size(), info.Mode(), info.IsDir()))
	}
	backupPath := filepath.Join(filepath.Dir(hostsPath), "hosts.bak")
	commonLogger.Println(commonUi.Detail("Hosts backup path: %q", backupPath))
	if info, statErr := os.Stat(backupPath); statErr != nil {
		commonLogger.Println(commonUi.Detail("Hosts backup stat: %v", statErr))
	} else {
		commonLogger.Println(commonUi.Detail("Hosts backup info: size=%d mode=%v isDir=%v", info.Size(), info.Mode(), info.IsDir()))
		commonLogger.Println(commonUi.Detail("If the backup already exists from a previous failed run, delete it after verifying its contents and retry."))
	}
}

func runSetUp(args []string) (err error, exitCode int) {
	values, fs := admin.SetupFlagSet()
	if err = fs.Parse(args); err != nil {
		exitCode = common.ErrSyntax
		return
	}

	// validate required flags
	if values.GameId == "" {
		return errors.New("required flag 'game' not set"), common.ErrSyntax
	}
	common.SetUseInternet(values.CanUseInternet)

	internal.SetUp = new(true)
	if values.LogRoot != "" {
		if initErr := initializeFn(values.LogRoot); initErr != nil {
			commonLogger.Println(commonUi.Fail("Failed to initialize file logging: %s", initErr))
		}
	}
	trustedCertificate := false
	if len(values.AddLocalCertData) > 0 {
		commonLogger.Println(commonUi.Step("Adding local certificate"))
		crt := bytesToCertFn(values.AddLocalCertData)
		if crt == nil {
			commonLogger.Println(commonUi.Fail("Failed to parse certificate"))
			exitCode = internal.ErrLocalCertAddParse
			return
		}
		if err = trustCertsFn(false, []*x509.Certificate{crt}); err == nil {
			commonLogger.Println(commonUi.Ok("Successfully added local certificate"))
			trustedCertificate = true
			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				_, ok := <-sigs
				if ok {
					untrustCertificate()
					exitCode = common.ErrSignal
				}
			}()
		} else {
			commonLogger.Println(commonUi.Fail("Failed to add local certificate"))
			commonLogger.Println(commonUi.Fault("Error: %s", err))
			exitCode = internal.ErrLocalCertAdd
			return
		}
	}
	if len(values.MapIp) > 0 {
		commonLogger.Println(commonUi.Step("Adding IP mappings for game %s with IP %s...", values.GameId, values.MapIp.String()))
		if ok, addHostsErr := addHostsFn(values.MapIp, values.GameId, "", "", values.MacOsExclusiveMappings, hosts.FlushDns); ok {
			commonLogger.Println(commonUi.Ok("Successfully added IP mappings"))
		} else {
			exitCode = internal.ErrIpMapAdd
			commonLogger.Println(commonUi.Fail("Failed to add IP mappings"))
			if addHostsErr != nil {
				commonLogger.Println(commonUi.Fault("Error message: %s", addHostsErr))
			} else {
				commonLogger.Println(commonUi.Fault("Error message: unknown (AddHosts returned ok=false with nil error)"))
			}
			logHostsDiagnostics()
			if trustedCertificate {
				if !untrustCertificate() {
					exitCode = internal.ErrIpMapAddRevert
				}
			}
		}
	}
	return
}
