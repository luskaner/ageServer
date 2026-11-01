package cmd

import (
	"io"
	"net"
	"net/http"
	"net/netip"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor"
	"github.com/luskaner/ageServer/common/fileLock"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/common/uuid"
	"github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/ip"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/initializer"
	"github.com/spf13/pflag"
)

var (
	fileLockNewFn            = func() *fileLock.PidLock { return &fileLock.PidLock{} }
	fileLockLockFn           = func(l *fileLock.PidLock) error { return l.Lock() }
	fileLockUnlockFn         = func(l *fileLock.PidLock) error { return l.Unlock() }
	commonLoggerInitFn       = commonLogger.Initialize
	commonLoggerCloseFn      = commonLogger.CloseFileLog
	loggerOpenMainFileLogFn  = logger.OpenMainFileLog
	isAdminFn                = executor.IsAdmin
	dnsConnectivityFn        = common.DNSConnectivity
	cacheNetworkInterfacesFn = models.CacheNetworkInterfaces
	uuidParseFn              = uuid.Parse
	initConfigFn             = initConfig
	resolveHostsFn           = ip.ResolveHosts
	initializeGameFn         = initializer.InitializeGame
	certificatePairFolderFn  = common.CertificatePairFolder
	// Checked at startup for the same reason the TLS material is: without it the
	// server cannot produce the signatures the client validates.
	signingKeyPairCheckFn    = models.CheckSigningKeyPair
	queryConnectionsFn       = ip.QueryConnections
	listenQueryConnectionsFn = ip.ListenQueryConnections
	newHTTPServerFn          = func(addr string, handler http.Handler) *http.Server {
		return &http.Server{Addr: addr, Handler: handler}
	}
	initializeRngFn = internal.InitializeRng
	_               = net.ParseIP
	_               = io.Writer(nil)
	_               = pflag.FlagSet{}
	_               = mapset.NewThreadUnsafeSet[string]
	_               = netip.Addr{}
)
