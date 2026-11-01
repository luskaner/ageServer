package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/v2"
	"github.com/luskaner/ageServer/common/cmd/server"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/uuid"
	"github.com/spf13/pflag"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/cmd"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/common/paths"
	"github.com/luskaner/ageServer/launcher-common/ui"
	"github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/embedded"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
	"github.com/luskaner/ageServer/server/internal/models/initializer"
	"github.com/luskaner/ageServer/server/internal/routes/router"
)

var configPaths = []string{paths.ConfigsPath, "."}
var values *server.Values

var (
	Version              string
	authenticationValues = mapset.NewThreadUnsafeSet[string]("required", "cached", "adaptive", "disabled")
)

func Execute() (err error, exitCode int) {
	var singleFs *cmd.SingleFlagSet
	values, singleFs = server.SingleFlagSet(Version, configPaths, runRoot)
	return singleFs.Execute()
}

func runRoot(fs *pflag.FlagSet) (err error, exitCode int) {
	lock := fileLockNewFn()
	if err = fileLockLockFn(lock); err != nil {
		logger.Fail("Failed to lock pid file. Kill process server if it is running in your task manager.")
		logger.Fault("%s", err.Error())
		commonLoggerCloseFn()
		exitCode = common.ErrPidLock
		return
	}
	cfg, usedFile := initConfigFn(fs)
	commonLoggerInitFn(nil)
	if values.LogRoot == "" {
		values.LogRoot = commonLogger.LogRootDate("")
	}
	if err = loggerOpenMainFileLogFn(values.LogRoot, cfg.Log); err != nil {
		logger.Fail("Failed to open main log file: %s", err)
		exitCode = common.ErrFileLog
		return
	}
	// Same shape as the launcher: a one line header, then the phases. The config
	// file it used is named once, here, instead of being announced again later.
	ui.Banner(executables.Server, Version)
	ui.Section("Configuration")
	if usedFile != "" {
		ui.KV(0, "config file", usedFile)
		logger.PrintFile("config", usedFile)
	}
	if !cfg.Internet.Enabled {
		internal.CanUseInternet = false
		logger.Info("Internet usage is disabled via config.")
	} else {
		internal.CanUseInternet = dnsConnectivityFn()
		cacheNetworkInterfacesFn(cfg.Internet.IP)
	}
	if !internal.CanUseInternet {
		logger.Warn("No internet connectivity, some features will fallback gracefully.")
	}
	if !authenticationValues.ContainsOne(cfg.Authentication) {
		logger.Fail("Invalid authentication value: %s", cfg.Authentication)
		exitCode = internal.ErrInvalidAuthentication
		return
	} else if cfg.Authentication == "required" && !internal.CanUseInternet {
		logger.Fail("Authentication is set to required but there is no internet connectivity, which is required for authentication. Change the authentication method or fix the connectivity.")
		exitCode = internal.ErrInvalidAuthentication
		return
	}
	if cfg.Authentication == "adaptive" {
		if internal.CanUseInternet {
			cfg.Authentication = "cached"
		} else {
			cfg.Authentication = "disabled"
		}
		logger.Info("Adaptive authentication resolved to %s based on connectivity", cfg.Authentication)
	}
	if cfg.Authentication == "disabled" {
		logger.Warn("Authentication is disabled, you are responsible that users access it legally.")
	} else if cfg.GeneratePlatformUserId {
		logger.Fail("Generating a platform User ID is not compatible with the Authentication resolving to a value other than disabled.")
		exitCode = internal.ErrInvalidAuthentication
		return
	}
	internal.Authentication = cfg.Authentication
	var seed uint64
	if !values.Deterministic {
		seed = uint64(time.Now().UnixNano())
	}
	initializeRngFn(seed)
	if values.Id == "" {
		values.Id = uuid.New().String()
	}
	var closables []io.Closer
	defer func() {
		if r := recover(); r != nil {
			logger.Fail("%s", r)
			logger.Fault("%s", string(debug.Stack()))
			exitCode = common.ErrGeneral
		}
		commonLoggerCloseFn()
		for _, f := range closables {
			_ = f.Close()
		}
		// Stop the custom-file sweepers. Their payloads stay in the system temp
		// folder until CustomFiles.Expiry removes them, so a restart within the
		// TTL is not what discards a pending savegame.
		for _, g := range initializer.Games {
			if cf := g.CustomFiles(); cf != nil {
				cf.Close()
			}
		}
		_ = fileLockUnlockFn(lock)
	}()
	if internal.Id, err = uuidParseFn(values.Id); err != nil {
		logger.Fail("Invalid server instance ID")
		exitCode = internal.ErrInvalidId
		return
	}
	logger.Info("Server instance ID: %s", internal.Id)
	if cfg.GeneratePlatformUserId {
		logger.Warn("Generating platform User ID, this should only be used as a last resort and the custom launcher should be properly configured instead.")
	}
	gameSet := mapset.NewThreadUnsafeSet[string](cfg.Games.Enabled...)
	if gameSet.IsEmpty() {
		logger.Fail("No games specified")
		exitCode = internal.ErrGames
		return
	}
	for g := range gameSet.Iter() {
		if !game.SupportedGames.ContainsOne(g) {
			logger.Fail("Invalid game specified: %s", g)
			exitCode = internal.ErrGames
			return
		}
	}
	if isAdminFn() {
		logger.Warn("Running as administrator, this is not recommended for security reasons.")
		if runtime.GOOS == "linux" {
			logger.Detail("If the issue is that you cannot listen on the port, then run `sudo setcap CAP_NET_BIND_SERVICE=+eip %s`, before re-running the server", os.Args[0])
		}
	}
	certificatePairFolder := certificatePairFolderFn(os.Args[0])
	if certificatePairFolder == "" {
		logger.Fail("Failed to determine certificate pair folder")
		exitCode = internal.ErrCertDirectory
		return
	}
	// Only asked for when this run will actually sign: with Sign off the
	// captured signatures in resources/responses go out untouched and no key
	// is read at all, so an install without one is complete rather than broken.
	// It reads cfg.Sign.Enabled and not internal.SignConfigRuntime.Enabled: the
	// runtime copy is assigned further down, so reading it here made the check
	// see the previous run's value (false on a fresh process) and never demand
	// the key it was configured to demand.
	if cfg.Sign.Enabled {
		if keyErr := signingKeyPairCheckFn(certificatePairFolder); keyErr != nil {
			logger.Fail("Failed to load the signing key pair")
			logger.Fault("%v", keyErr)
			exitCode = internal.ErrSigningKey
			return
		}
	}
	announceEnabled := cfg.Announcement.Enabled
	multicastGroups := mapset.NewThreadUnsafeSet[netip.Addr]()
	if announceEnabled && cfg.Announcement.Multicast {
		var multicastIP netip.Addr
		multicastIP, err = netip.ParseAddr(cfg.Announcement.MulticastGroup)
		if err != nil || !multicastIP.Is4() || !multicastIP.IsMulticast() {
			logger.Fail("Invalid multicast IP: %s", cfg.Announcement.MulticastGroup)
			if err != nil {
				logger.Fault("%s", err.Error())
			}
			exitCode = internal.ErrMulticastGroup
			return
		}
		multicastGroups.Add(multicastIP)
	}
	announcePort := cfg.Announcement.Port
	internal.AnnounceMessageData = make(map[string]common.AnnounceMessageData002, gameSet.Cardinality())
	internal.GeneratePlatformUserId = cfg.GeneratePlatformUserId
	internal.SignConfigRuntime.Enabled = cfg.Sign.Enabled
	internal.SignConfigRuntime.PlatformUserIdExceptions = cfg.Sign.PlatformUserIdExceptions
	if err = applyCustomFilesConfig(cfg); err != nil {
		logger.Fail("Invalid CustomFiles configuration: %s", err)
		exitCode = internal.ErrCustomFiles
		return
	}
	if err = applyBattleServerReplaysConfig(cfg); err != nil {
		logger.Fail("Invalid BattleServerReplays configuration: %s", err)
		exitCode = internal.ErrBattleServerReplays
		return
	}
	var servers []*http.Server
	internal.InitializeStopSignal()

	// Start embedded battle servers for each game before initializing the game.
	// Auto-enable embedded battle server if no other battle server (config or discovered) exists.
	var embeddedInstances []*embedded.Instance
	for gameId := range gameSet.Iter() {
		// Check if there are any configured battle servers in config.toml
		configBattleServers := cfg.GetGameBattleServers(gameId)
		// Check if there are any battle servers discovered by battle-server-manager
		discoveredBattleServers, discErr := battleServer.Configs(gameId, true, true)
		if discErr != nil {
			logger.Fail("Failed to read discovered battle servers for %s: %s", gameId, discErr)
			exitCode = internal.ErrEmbeddedBattleServer
			return
		}

		hasExternalBattleServer := len(configBattleServers) > 0 || len(discoveredBattleServers) > 0
		ebs := cfg.GetGameEmbeddedBattleServer(gameId)

		// StartIfMissing controls auto-start behavior when no external battle server exists.
		// Values: "true" (default), "false", "required" (true for age4/athens, false for others).
		// First check CLI flag (applies to all games), then config file.
		startIfMissing := strings.ToLower(strings.TrimSpace(values.StartBsIfMissing))
		hasExplicitStartIfMissing := strings.TrimSpace(values.StartBsIfMissing) != ""

		// If not set via CLI, check config file
		if !hasExplicitStartIfMissing {
			startIfMissing = strings.ToLower(strings.TrimSpace(ebs.StartIfMissing))
			hasExplicitStartIfMissing = strings.TrimSpace(ebs.StartIfMissing) != ""
		}

		if !hasExplicitStartIfMissing {
			startIfMissing = "true"
		}

		// Evaluate StartIfMissing value
		shouldAutoStart := false
		switch startIfMissing {
		case "true":
			shouldAutoStart = true
		case "false":
			shouldAutoStart = false
		case "required":
			// Required games: age4 and athens (AoM)
			switch gameId {
			case game.AoE4, game.AoM:
				shouldAutoStart = true
			default:
				shouldAutoStart = false
			}
		default:
			logger.Warn("Unknown StartIfMissing value %q for %s, defaulting to true", ebs.StartIfMissing, gameId)
			shouldAutoStart = true
		}

		shouldStart := shouldAutoStart && !hasExternalBattleServer

		if shouldStart {
			// Use configured values or defaults matching battle-server-manager
			region := ebs.Region
			if region == "" {
				region = "Server"
			}
			name := ebs.Name
			if name == "" {
				name = "Server"
			}
			ipv4 := ebs.IPv4
			if ipv4 == "" {
				ipv4 = "auto"
			}

			inst := &embedded.Instance{
				Opts: embedded.Options{
					GameID:        gameId,
					Region:        region,
					Name:          name,
					IPv4:          ipv4,
					BsPort:        ebs.BsPort,
					WebSocketPort: ebs.WebSocketPort,
					OutOfBandPort: ebs.OutOfBandPort,
					LogLevel:      ebs.LogLevel,
				},
			}
			replayOpts := embedded.ReplayOptions{
				Enabled:      internal.BattleServerReplaysEnabled,
				Folder:       internal.BattleServerReplaysFolder,
				MaxFileSize:  internal.BattleServerReplaysMaxFileSize,
				MaxTotalSize: internal.BattleServerReplaysMaxTotalSize,
				Expiry:       internal.BattleServerReplaysExpiry,
			}
			// Not a timeout: the embedded server owns a context of its own and
			// keeps running until inst.Stop, because the context it is started
			// with is its lifetime (see embedded.Instance.Start). A startup
			// budget here would cancel that lifetime as soon as Start returned.
			startErr := inst.Start(context.Background(), certificatePairFolder, replayOpts)
			if startErr != nil {
				logger.Fail("Failed to start embedded battle server for %s: %s", gameId, startErr)
				exitCode = internal.ErrEmbeddedBattleServer
				return
			}
			embeddedInstances = append(embeddedInstances, inst)
			if !hasExplicitStartIfMissing {
				logger.Info("Auto-started embedded battle server for %s (StartIfMissing=%s, no external battle server found)", gameId, startIfMissing)
			}
		}
	}

	for gameId := range gameSet.Iter() {
		ui.Section("Game " + gameId)
		hosts := cfg.GetGameHosts(gameId)
		addrs := resolveHostsFn(mapset.NewThreadUnsafeSet[string](hosts...))
		if addrs.IsEmpty() {
			logger.Fail("Failed to resolve host (or it was an IPv6 address)")
			exitCode = internal.ErrResolveHost
			return
		}
		if err = initializeGameFn(gameId, cfg.GetGameBattleServers(gameId)); err != nil {
			logger.Fail("Failed to initialize game: %s", err)
			exitCode = internal.ErrGame
			return
		}
		if battlesServers, ok := models.BattleServersStore[gameId]; ok && len(battlesServers) > 0 {
			logger.Info("Battle Servers:")
			for _, bs := range battlesServers {
				logger.Detail("%s", bs.String())
			}
		}
		var writer io.Writer
		var root *commonLogger.Root
		gameLogRoot := values.LogRoot
		var filePrefix string
		if values.Flatlog {
			filePrefix = fmt.Sprintf("%s_", gameId)
		} else {
			gameLogRoot = filepath.Join(gameLogRoot, gameId)
		}
		customLoggerWriters := []io.Writer{os.Stderr}
		if commonLogger.FileLogger == nil {
			writer = os.Stdout
		} else {
			customLoggerWriters = append(customLoggerWriters, &commonLogger.Buf)
			var f *os.File
			if err, root = commonLogger.NewFile(gameLogRoot, "", true); err != nil {
				logger.Fail("Failed to prepare log folder: %s", err)
				exitCode = internal.ErrCreateLogFile
				return
			} else if f, err = root.Open(filePrefix + "access_log"); err != nil {
				logger.Fail("Failed to open access log file: %s", err)
				exitCode = internal.ErrCreateLogFile
				return
			}
			closables = append(closables, f)
			writer = f
		}
		customLogger := log.New(
			&internal.CustomWriter{OriginalWriter: io.MultiWriter(customLoggerWriters...)},
			"|SERVER| ",
			log.Ltime|log.Lmicroseconds,
		)
		internal.AnnounceMessageData[gameId] = internal.AnnounceMessageDataLatest{
			GameTitle: gameId,
			Version:   Version,
		}
		general := &router.General{Writer: writer}
		mux := general.InitializeRoutes(gameId, router.HostMiddleware(gameId, writer))
		mux = router.TitleMiddleware(gameId, mux)
		if root != nil {
			var f *os.File
			if f, err = root.Open(filePrefix + "communication_log"); err != nil {
				logger.Fail("Failed to open communication log file: %s", err)
				exitCode = internal.ErrCreateLogFile
			} else {
				closables = append(closables, logger.NewBuffer(f))
				mux = router.NewLoggingMiddleware(mux)
			}
		}
		// 1 MB limit
		mux = router.MaxBodySizeMiddleware(1<<20, mux)
		for addr := range addrs.Iter() {
			var certFile string
			var keyFile string
			if common.SelfSignedCertGame(gameId) {
				certFile = filepath.Join(certificatePairFolder, common.SelfSignedCert)
				keyFile = filepath.Join(certificatePairFolder, common.SelfSignedKey)
			} else {
				certFile = filepath.Join(certificatePairFolder, common.Cert)
				keyFile = filepath.Join(certificatePairFolder, common.Key)
			}
			var listenConns []*net.UDPConn
			if announceEnabled {
				err, listenConns = queryConnectionsFn(addr, multicastGroups, announcePort)
				if err != nil {
					logger.Fail("Failed to listen to UDP connections for address %s", addr)
					exitCode = internal.ErrAnnounce
					return
				}
			}
			s := newHTTPServerFn(addr.String()+":443", mux)
			s.ErrorLog = customLogger
			s.IdleTimeout = time.Second * 30
			s.ReadTimeout = time.Second * 5
			s.WriteTimeout = time.Second * 30
			s.MaxHeaderValueCount = 64

			logger.Ok("Listening on %s", s.Addr)
			go func() {
				if len(listenConns) > 0 {
					for _, conn := range listenConns {
						logger.Detail("Listening for query connections on %s", conn.LocalAddr())
					}
					listenQueryConnectionsFn(listenConns)
				}
				err = s.ListenAndServeTLS(certFile, keyFile)
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Fail("Failed to start server")
					logger.Fault("%s", err)
					exitCode = internal.ErrStartServer
					return
				}
			}()
			servers = append(servers, s)
		}
	}

	<-internal.StopSignal

	ui.Section("Teardown")
	logger.Step("Servers are shutting down...")

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	for _, s := range servers {
		wg.Go(func() {
			if err = s.Shutdown(ctx); err != nil {
				logger.Fail("Server %s forced to shutdown: %s", s.Addr, err)
			}
			logger.Ok("Server %s stopped", s.Addr)
		})
	}
	wg.Wait()

	// Stop embedded battle servers.
	for _, inst := range embeddedInstances {
		inst.Stop()
	}

	logger.Step("Flushing pending user data...")
	flushCtx, flushCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer flushCancel()
	done := make(chan struct{})
	go func() {
		for _, g := range initializer.Games {
			if users := g.Users(); users != nil {
				users.FlushAll()
			}
		}
		close(done)
	}()
	select {
	case <-done:
		logger.Ok("All pending writes flushed")
	case <-flushCtx.Done():
		logger.Fail("Flush timed out after %v: %v", 2*time.Second, flushCtx.Err())
	}
	return
}

func initConfig(fs *pflag.FlagSet) (*internal.Configuration, string) {
	k := koanf.New(".")
	defaults := map[string]any{
		"Log":                              false,
		"GeneratePlatformUserId":           false,
		"Sign.Enabled":                     false,
		"Sign.PlatformUserIdExceptions":    []string{},
		"Authentication":                   "disabled",
		"Internet.Enabled":                 true,
		"Internet.IP":                      "auto",
		"Announcement.Enabled":             true,
		"Announcement.Multicast":           true,
		"Announcement.MulticastGroup":      common.AnnounceMulticastGroup,
		"Announcement.Port":                common.AnnouncePort,
		"Games.Enabled":                    []string{},
		"CustomFiles.Enabled":              true,
		"CustomFiles.Folder":               "auto",
		"CustomFiles.MaxFileSize":          "32MB",
		"CustomFiles.MaxTotalSize":         "256MB",
		"CustomFiles.Expiry":               "12h",
		"BattleServerReplays.Enabled":      false,
		"BattleServerReplays.Folder":       "auto",
		"BattleServerReplays.MaxFileSize":  "32MB",
		"BattleServerReplays.MaxTotalSize": "256MB",
		"BattleServerReplays.Expiry":       "12h",
	}
	for g := range game.SupportedGames.Iter() {
		defaults[fmt.Sprintf("Games.%s.Hosts", g)] = []string{netip.IPv4Unspecified().String()}
		defaults[fmt.Sprintf("Games.%s.EmbeddedBattleServer.StartIfMissing", g)] = "true"
	}
	bindings := map[string]string{
		"log":                    "Log",
		"generatePlatformUserId": "GeneratePlatformUserId",
		"authentication":         "Authentication",
		"internet":               "Internet.Enabled",
		"externalIPAddress":      "Internet.IP",
		"announce":               "Announcement.Enabled",
		"announceMulticast":      "Announcement.Multicast",
		"announceMulticastGroup": "Announcement.MulticastGroup",
		"announcePort":           "Announcement.Port",
		cmd.GamesIdentifier:      "Games.Enabled",
	}
	var fileCandidates []string
	if values.CfgFile != "" {
		fileCandidates = append(fileCandidates, values.CfgFile)
	} else {
		for _, configPath := range configPaths {
			fileCandidates = append(fileCandidates, filepath.Join(configPath, "config.toml"))
		}
	}

	usedFile := common.LoadKoanfLayersOrExit(k, defaults, fileCandidates, toml.Parser(), fs, bindings, executables.Server, commonLogger.Println)
	if values.CfgFile != "" && usedFile == "" {
		logger.Warn("No config file found, using defaults.")
	}
	// The file this run is using is named by the summary at the top, and only
	// there: announced once, from one place.

	var c internal.Configuration
	if err := k.Unmarshal("", &c); err != nil {
		logger.Fail("Unable to decode configuration: %s", err)
		os.Exit(common.ErrConfigParse)
	}
	return &c, usedFile
}

// applyCustomFilesConfig validates the CustomFiles section and publishes the
// resolved values in the internal runtime globals. Sizes accept the usual
// B/KB/MB/GB suffixes; the TTL is a Go duration such as 12h or 5m.
func applyCustomFilesConfig(cfg *internal.Configuration) error {
	internal.CustomFilesEnabled = cfg.CustomFiles.Enabled
	internal.CustomFilesFolder = cfg.CustomFiles.Folder
	if !cfg.CustomFiles.Enabled {
		// Nothing reads the limits or the TTL while the endpoints are off, so a
		// typo in them must not stop a server that does not use the feature.
		return nil
	}
	maxFileSize, err := parseByteSize(cfg.CustomFiles.MaxFileSize)
	if err != nil {
		return fmt.Errorf("MaxFileSize: %w", err)
	}
	maxTotalSize, err := parseByteSize(cfg.CustomFiles.MaxTotalSize)
	if err != nil {
		return fmt.Errorf("MaxTotalSize: %w", err)
	}
	if maxFileSize <= 0 {
		return fmt.Errorf("MaxFileSize must be greater than zero")
	}
	if maxTotalSize <= 0 {
		return fmt.Errorf("MaxTotalSize must be greater than zero")
	}
	if maxFileSize > maxTotalSize {
		return fmt.Errorf("MaxFileSize (%d) must not exceed MaxTotalSize (%d)", maxFileSize, maxTotalSize)
	}
	expiry, err := time.ParseDuration(strings.TrimSpace(cfg.CustomFiles.Expiry))
	if err != nil {
		return fmt.Errorf("Expiry: %w", err)
	}
	if expiry <= 0 {
		return fmt.Errorf("Expiry must be greater than zero")
	}
	internal.CustomFilesMaxFileSize = maxFileSize
	internal.CustomFilesMaxTotalSize = maxTotalSize
	internal.CustomFilesExpiry = expiry
	return nil
}

func parseByteSize(value string) (int64, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return 0, errors.New("empty size")
	}
	units := []struct {
		suffix     string
		multiplier int64
	}{
		{"GB", 1 << 30},
		{"G", 1 << 30},
		{"MB", 1 << 20},
		{"M", 1 << 20},
		{"KB", 1 << 10},
		{"K", 1 << 10},
		{"B", 1},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(value, unit.suffix))
			n, err := strconv.ParseInt(number, 10, 64)
			if err != nil {
				return 0, err
			}
			return n * unit.multiplier, nil
		}
	}
	return strconv.ParseInt(value, 10, 64)
}

// applyBattleServerReplaysConfig validates the BattleServerReplays section and publishes
// the resolved values in the internal runtime globals. Sizes accept the usual
// B/KB/MB/GB suffixes; the TTL is a Go duration such as 12h or 5m.
func applyBattleServerReplaysConfig(cfg *internal.Configuration) error {
	internal.BattleServerReplaysEnabled = cfg.BattleServerReplays.Enabled
	internal.BattleServerReplaysFolder = cfg.BattleServerReplays.Folder
	if !cfg.BattleServerReplays.Enabled {
		return nil
	}
	maxFileSize, err := parseByteSize(cfg.BattleServerReplays.MaxFileSize)
	if err != nil {
		return fmt.Errorf("MaxFileSize: %w", err)
	}
	maxTotalSize, err := parseByteSize(cfg.BattleServerReplays.MaxTotalSize)
	if err != nil {
		return fmt.Errorf("MaxTotalSize: %w", err)
	}
	if maxFileSize <= 0 {
		return fmt.Errorf("MaxFileSize must be greater than zero")
	}
	if maxTotalSize <= 0 {
		return fmt.Errorf("MaxTotalSize must be greater than zero")
	}
	if maxFileSize > maxTotalSize {
		return fmt.Errorf("MaxFileSize (%d) must not exceed MaxTotalSize (%d)", maxFileSize, maxTotalSize)
	}
	expiry, err := time.ParseDuration(strings.TrimSpace(cfg.BattleServerReplays.Expiry))
	if err != nil {
		return fmt.Errorf("Expiry: %w", err)
	}
	// Expiry <= 0 means keep forever (no TTL-based expiration)
	internal.BattleServerReplaysMaxFileSize = maxFileSize
	internal.BattleServerReplaysMaxTotalSize = maxTotalSize
	internal.BattleServerReplaysExpiry = expiry
	return nil
}
