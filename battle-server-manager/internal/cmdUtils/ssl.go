package cmdUtils

import (
	"fmt"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmdlog"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/common/game"
)

var (
	certificatePairs = common.CertificatePairs
	findServerPath   = executables.FindPath
)

func keyCert(parentFolder, gameId string) (resolvedCertFile string, resolvedKeyFile string, err error) {
	ok, cert, key, _, selfSignedCert, selfSignedKey := certificatePairs(parentFolder)
	if !ok {
		err = fmt.Errorf("no SSL certificate and keys found")
		return
	}
	if gameId == game.AoE4 || gameId == game.AoM {
		resolvedCertFile = cert
		resolvedKeyFile = key
	} else {
		resolvedCertFile = selfSignedCert
		resolvedKeyFile = selfSignedKey
	}
	return
}

func ResolveSSLFilesPath(gameId string, certsPath string) (resolvedCertFile string, resolvedKeyFile string, err error) {
	if certsPath == "auto" {
		cmdlog.Step("Auto resolving SSL certificate and key files...")
		serverExe := findServerPath(executables.NativeFileName(true, executables.Server))
		if serverExe == "" {
			err = fmt.Errorf("could not find server executable")
			return
		}
		certsPath = common.CertificatePairFolder(serverExe)
	}
	return keyCert(certsPath, gameId)
}
