package cmd

import (
	"runtime"

	"github.com/luskaner/ageServer/common"
	launcherCommonHosts "github.com/luskaner/ageServer/common/hosts"
	"github.com/luskaner/ageServer/launcher-common/cert"
	"github.com/luskaner/ageServer/launcher-config-admin/internal"
	"github.com/luskaner/ageServer/launcher-config-admin/internal/hosts"
)

var (
	bytesToCertFn  = common.BytesToCertificate
	trustCertsFn   = cert.TrustCertificates
	untrustCertsFn = cert.UntrustCertificates
	addHostsFn     = launcherCommonHosts.AddHosts
	removeHostsFn  = hosts.RemoveHosts
	flushDnsFn     = hosts.FlushDns
	flushCertsFn   = cert.FlushCerts
	initializeFn   = internal.Initialize
	runtimeGOOS    = runtime.GOOS
)
