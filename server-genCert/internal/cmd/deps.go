package cmd

import (
	"os"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/server-genCert/internal"
)

var (
	osExecutableFn             = os.Executable
	certificatePairFolderFn    = common.CertificatePairFolder
	certificatePairsFn         = common.CertificatePairs
	generateCertificatePairsFn = internal.GenerateCertificatePairs
)
