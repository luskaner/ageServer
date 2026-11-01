package cmd

import (
	"crypto/x509"
	"os"
	"os/signal"
	"syscall"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/cmd/config/admin"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/launcher-config-admin/internal"
)

func trustCertificates(certificates []*x509.Certificate) bool {
	commonLogger.Println(commonUi.Step("Adding previously removed local certificate"))
	if err := trustCertsFn(false, certificates); err == nil {
		commonLogger.Println(commonUi.Ok("Successfully added local certificate"))
		return true
	}
	commonLogger.Println(commonUi.Fail("Failed to add local certificate"))
	return false
}

func runRevert(args []string) (err error, exitCode int) {
	values, fs := admin.RevertFlagSet()
	if err = fs.Parse(args); err != nil {
		exitCode = common.ErrSyntax
		return
	}
	internal.SetUp = new(false)
	if values.LogRoot != "" {
		if initErr := initializeFn(values.LogRoot); initErr != nil {
			commonLogger.Println(commonUi.Fail("Failed to initialize file logging: %s", initErr))
		}
	}
	if values.RemoveAll {
		values.IPs = true
		values.Certs = true
	}
	var removedCertificates []*x509.Certificate
	if values.Certs {
		commonLogger.Println(commonUi.Step("Removing local certificate"))
		removedCertificates, err = untrustCertsFn(false)
		if err == nil {
			commonLogger.Println(commonUi.Ok("Successfully removed local certificate"))
			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				_, ok := <-sigs
				if ok {
					trustCertificates(removedCertificates)
					exitCode = common.ErrSignal
				}
			}()
		} else {
			commonLogger.Println(commonUi.Fail("Failed to remove local certificate"))
			commonLogger.Println(commonUi.Fault("Error: %s", err))
			if !values.RemoveAll {
				exitCode = internal.ErrLocalCertRemove
				return
			}
		}
	}
	if values.IPs {
		commonLogger.Println(commonUi.Step("Removing IP mappings"))
		if err = removeHostsFn(); err == nil {
			commonLogger.Println(commonUi.Ok("Successfully removed IP mappings"))
		} else {
			exitCode = internal.ErrIpMapRemove
			if !values.RemoveAll {
				if removedCertificates != nil {
					if !trustCertificates(removedCertificates) {
						exitCode = internal.ErrIpMapRemoveRevert
					}
				}
			}
			commonLogger.Println(commonUi.Fail("Failed to remove IP mappings"))
		}
	}
	return
}
