package cmd

import (
	"path/filepath"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/cmd/genCert"
	"github.com/luskaner/ageServer/common/cmdlog"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/server-genCert/internal"
	"github.com/spf13/pflag"
)

var (
	Version string
	values  *genCert.Values
)

func runRoot(_ *pflag.FlagSet) (err error, exitCode int) {
	var exe string
	exe, err = osExecutableFn()
	if err != nil {
		cmdlog.Fail("Could not get executable path")
		exitCode = common.ErrGeneral
		return
	}
	serverExe := filepath.Join(filepath.Dir(filepath.Dir(exe)), executables.NativeFileName(true, executables.Server))
	serverFolder := certificatePairFolderFn(serverExe)
	if serverFolder == "" {
		cmdlog.Fail("Failed to determine certificate pairs folder")
		exitCode = internal.ErrCertDirectory
		return
	}
	if !values.Replace {
		certificateFolder := certificatePairFolderFn(serverExe)
		if exists, _, _, _, _, _ := certificatePairsFn(certificateFolder); exists {
			cmdlog.Fail("Already have certificate pairs and replace is false, set replace to true or delete it manually.")
			if values.IgnoreIfExisting {
				return
			}
			exitCode = internal.ErrCertCreateExisting
			return
		}
	}
	if !generateCertificatePairsFn(serverFolder) {
		cmdlog.Fail("Could not generate certificate pair.")
		exitCode = internal.ErrCertCreate
		return
	}
	cmdlog.Ok("Certificate pair generated successfully.")
	return
}

func Execute() (err error, exitCode int) {
	var singleFs *cmd.SingleFlagSet
	values, singleFs = genCert.SingleFlagSet(Version, runRoot)
	return singleFs.Execute()
}
