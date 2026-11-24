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
	"slices"
	"strings"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/luskaner/ageServer/common/cmd/server"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/uuid"
	"github.com/spf13/pflag"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/bytesize"
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
	// The domain map is built from the generated release domains, so the flag has
	// to be set before it is built and before the games resolve their hosts.
	common.SetUseInternet(internal.CanUseInternet)
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
	assumeQueue := strings.ToLower(strings.TrimSpace(cfg.Automatch.AssumeQueue))
	if assumeQueue != "" && assumeQueue != "rm-solo" {
		logger.Fail("Invalid automatch assume-queue value: %s (accepted: rm-solo)", cfg.Automatch.AssumeQueue)
		exitCode = common.ErrConfigParse
		return
	}
	if assumeQueue != "" {
		logger.Warn("Automatch unknown matchTypes will be paired as unrated 1v1 Random Map (assume-queue=%s)", assumeQueue)
	}
	internal.AutomatchAssumeQueue = assumeQueue
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
		// Stop the replay sweepers and release the cloud roots. The uploaded
		// replays stay on disk: the TTL is what removes them.
		for _, g := range initializer.Games {
			if store := g.Resources().CloudFiles(); store != nil {
				store.Close()
			}
		}
		if internal.ModsAPI != nil {
			_ = internal.ModsAPI.Shutdown(context.Background())
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
	if err = applyGameOverrides(cfg, usedFile, fs, values, gameSet); err != nil {
		logger.Fail("Invalid game configuration: %s", err)
		exitCode = common.ErrConfigParse
		return
	}
	gameIds := gameSet.ToSlice()
	slices.Sort(gameIds)
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
	internal.ApiEnabled = cfg.Api.Enabled
	if !cfg.Api.Enabled && cfg.Mods.Enabled {
		logger.Fail("Mods.Enabled is true but Api.Enabled is false: the mods API lives in the API tree that is turned off.")
		exitCode = internal.ErrApiDisabledWithMods
		return
	}
	if err = applyCustomFilesConfig(cfg); err != nil {
		logger.Fail("Invalid CustomFiles configuration: %s", err)
		exitCode = internal.ErrCustomFiles
		return
	}
	if err = applyReplaysConfig(cfg); err != nil {
		logger.Fail("Invalid Replays configuration: %s", err)
		exitCode = internal.ErrReplays
		return
	}
	if err = applyModsConfig(cfg); err != nil {
		logger.Fail("Invalid Mods configuration: %s", err)
		exitCode = internal.ErrMods
		return
	}
	// One library instance serves both trees: the mods API and MatchContent.
	// It is built when either feature is on, so turning mods off does not take
	// the savegame endpoints with it.
	if internal.ModsEnabled || internal.CustomFilesConfig.Enabled {
		var mods internal.ModsServer
		if mods, err = newModsServerFn(internal.ModsConfig); err != nil {
			logger.Fail("Failed to start the embedded Age API: %s", err)
			if internal.ModsEnabled {
				exitCode = internal.ErrMods
			} else {
				exitCode = internal.ErrCustomFiles
			}
			return
		}
		internal.ModsAPI = mods
		if internal.ModsEnabled {
			logger.Ok("Mods API ready (%s)", internal.ModsConfig.DBPath)
		}
		if internal.CustomFilesConfig.Enabled {
			logger.Ok("MatchContent ready")
		}
	}
	var servers []*http.Server
	internal.InitializeStopSignal()

	// Start embedded battle servers for each game before initializing the game.
	// Auto-enable embedded battle server if no other battle server (config or discovered) exists.
	var embeddedInstances []*embedded.Instance
	for _, gameId := range gameIds {
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
		// applyGameOverrides has already resolved the per-game file and the flag.
		startIfMissing := strings.ToLower(strings.TrimSpace(ebs.StartIfMissing))
		if startIfMissing == "" {
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
		// Equivalent to the launcher's best-effort warning: with the value now
		// living in config.<game>.toml, the server is the only place that can
		// still know it. It does not block the start.
		if !hasExternalBattleServer && !shouldStart && (gameId == game.AoE4 || gameId == game.AoM) {
			logger.Warn("%s needs a battle server and StartIfMissing resolves to false: make sure one is running and configured, or the game cannot start a match", gameId)
		}

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
			// Not a timeout: the embedded server owns a context of its own and
			// keeps running until inst.Stop, because the context it is started
			// with is its lifetime (see embedded.Instance.Start). A startup
			// budget here would cancel that lifetime as soon as Start returned.
			startErr := inst.Start(context.Background(), certificatePairFolder)
			if startErr != nil {
				logger.Fail("Failed to start embedded battle server for %s: %s", gameId, startErr)
				exitCode = internal.ErrEmbeddedBattleServer
				return
			}
			embeddedInstances = append(embeddedInstances, inst)
			logger.Info("Auto-started embedded battle server for %s (StartIfMissing=%s, no external battle server found)", gameId, startIfMissing)
		}
	}

	// Phase A: initialise every game and build its mux. No listener is opened
	// yet, so a configuration error cannot leave half the process bound.
	gameMuxes := make(map[string]http.Handler, len(gameIds))
	gameModels := make(map[string]models.Game, len(gameIds))
	for _, gameId := range gameIds {
		ui.Section("Game " + gameId)
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
		if commonLogger.FileLogger == nil {
			writer = os.Stdout
		} else {
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
		internal.AnnounceMessageData[gameId] = internal.AnnounceMessageDataLatest{
			GameTitle: gameId,
			Version:   Version,
		}
		// Request order: MaxBodySize -> Logging -> Title -> Host. Title has to
		// be outside Host so models.Game exists before Playfab, the API trees,
		// the CDN, community and the game routes read models.G.
		mux := router.TitleMiddleware(gameId, router.HostMiddleware(gameId, writer))
		if root != nil {
			var f *os.File
			if f, err = root.Open(filePrefix + "communication_log"); err != nil {
				logger.Fail("Failed to open communication log file: %s", err)
				exitCode = internal.ErrCreateLogFile
				return
			} else {
				closables = append(closables, logger.NewBuffer(f))
				mux = router.NewLoggingMiddleware(mux)
			}
		}
		// 1 MB limit
		mux = router.MaxBodySizeMiddleware(1<<20, mux)
		gameMuxes[gameId] = mux
		gameModels[gameId] = initializer.Games[gameId]
	}

	// Phase B: one listener per address, shared by every game. The mode only
	// selects the certificate pair.
	modeFilter := common.CertMode(strings.ToLower(strings.TrimSpace(values.Mode)))
	if modeFilter != "" && modeFilter != common.CertModeModern && modeFilter != common.CertModeLegacy {
		logger.Fail("Invalid mode value: %s (accepted: modern, legacy)", values.Mode)
		exitCode = common.ErrConfigParse
		return
	}
	modernHosts := cfg.Hosts.Modern
	legacyHosts := cfg.Hosts.Legacy
	if modeFilter == common.CertModeModern {
		legacyHosts = nil
	} else if modeFilter == common.CertModeLegacy {
		modernHosts = nil
	}
	modernAddrs, resolveErr := resolveConfiguredHosts(modernHosts, "Modern")
	if resolveErr != nil {
		logger.Fail("Failed to resolve hosts: %s", resolveErr)
		exitCode = internal.ErrResolveHost
		return
	}
	legacyAddrs, resolveErr := resolveConfiguredHosts(legacyHosts, "Legacy")
	if resolveErr != nil {
		logger.Fail("Failed to resolve hosts: %s", resolveErr)
		exitCode = internal.ErrResolveHost
		return
	}
	if validateErr := validateListenerSets(modernAddrs, legacyAddrs); validateErr != nil {
		logger.Fail("Invalid listener configuration: %s", validateErr)
		exitCode = common.ErrConfigParse
		return
	}
	if modernAddrs.IsEmpty() && legacyAddrs.IsEmpty() {
		if modeFilter != "" {
			logger.Fail("No addresses to listen on with certificate mode %s", modeFilter)
		} else {
			logger.Fail("No addresses to listen on: set [Hosts] Modern and/or Legacy")
		}
		exitCode = internal.ErrResolveHost
		return
	}
	specs := listenerSpecs(modernAddrs, legacyAddrs)
	resolver := router.NewGameResolver(gameSet, cfg.Api.Enabled, gameModels)
	dispatcher := router.NewDispatcher(gameMuxes, resolver)

	rootLogWriter := io.Writer(os.Stderr)
	if commonLogger.FileLogger != nil {
		rootLogWriter = io.MultiWriter(os.Stderr, &commonLogger.Buf)
	}
	rootLogger := log.New(
		&internal.CustomWriter{OriginalWriter: rootLogWriter},
		"|ROOT| ",
		log.Ltime|log.Lmicroseconds,
	)
	var udpConns []*net.UDPConn
	for _, spec := range specs {
		certFile := filepath.Join(certificatePairFolder, common.Cert)
		keyFile := filepath.Join(certificatePairFolder, common.Key)
		if spec.Mode == common.CertModeLegacy {
			certFile = filepath.Join(certificatePairFolder, common.SelfSignedCert)
			keyFile = filepath.Join(certificatePairFolder, common.SelfSignedKey)
		}
		var listenConns []*net.UDPConn
		if announceEnabled {
			err, listenConns = queryConnectionsFn(spec.Addr, multicastGroups, announcePort)
			if err != nil {
				logger.Fail("Failed to listen to UDP connections for address %s", spec.Addr)
				exitCode = internal.ErrAnnounce
				return
			}
			udpConns = append(udpConns, listenConns...)
		}
		s := newHTTPServerFn(spec.String(), router.RootRoutes(spec.Mode, nil, dispatcher, resolver.Resolve))
		s.ErrorLog = rootLogger
		s.IdleTimeout = time.Second * 30
		s.ReadTimeout = time.Second * 5
		s.WriteTimeout = time.Second * 30
		s.MaxHeaderValueCount = 64

		logger.Ok("Listening on %s (certificates: %s)", s.Addr, spec.Mode)
		go func(spec listenerSpec, s *http.Server, certFile, keyFile string, listenConns []*net.UDPConn) {
			if len(listenConns) > 0 {
				for _, conn := range listenConns {
					logger.Detail("Listening for query connections on %s", conn.LocalAddr())
				}
				listenQueryConnectionsFn(listenConns)
			}
			serveErr := s.ListenAndServeTLS(certFile, keyFile)
			if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				logger.Fail("Failed to start server")
				logger.Fault("%s", serveErr)
				exitCode = internal.ErrStartServer
				return
			}
		}(spec, s, certFile, keyFile, listenConns)
		servers = append(servers, s)
	}

	// Phase C: say what is listening and how each game stands against it.
	for _, line := range buildListenerReport(specs, gameIds, modeFilter) {
		logger.Println(line)
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

	// Close the announce sockets explicitly: their read loops close them when
	// they see the error, and this is what makes the second start of the test
	// not find "address already in use".
	for _, conn := range udpConns {
		_ = conn.Close()
	}

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
		"Log":                           false,
		"GeneratePlatformUserId":        false,
		"Sign.Enabled":                  false,
		"Sign.PlatformUserIdExceptions": []string{},
		"Authentication":                "disabled",
		"Internet.Enabled":              true,
		"Internet.IP":                   "auto",
		"Announcement.Enabled":          true,
		"Announcement.Multicast":        true,
		"Announcement.MulticastGroup":   common.AnnounceMulticastGroup,
		"Announcement.Port":             common.AnnouncePort,
		"Games.Enabled":                 []string{},
		"Hosts.Modern":                  []string{netip.IPv4Unspecified().String()},
		"Hosts.Legacy":                  []string{},
		"CustomFiles.Enabled":           true,
		"CustomFiles.Folder":            "resources/custom-files",
		"CustomFiles.MaxFileSize":       "32MB",
		"CustomFiles.MaxTotalSize":      "256MB",
		"CustomFiles.Expiry":            "12h",
		"Replays.Enabled":               true,
		"Replays.MaxFileSize":           "32MB",
		"Replays.MaxTotalSize":          "1GB",
		"Replays.Expiry":                "-1s",
		"Mods.Enabled":                  true,
		"Api.Enabled":                   true,
		"Mods.DBPath":                   "resources/mods/mods.db",
		"Mods.UploadDir":                "resources/mods/uploads",
		"Mods.MaxUploadSize":            "256MB",
		"Mods.RequireAuthForRead":       false,
		"Automatch.AssumeQueue":         "",
	}
	for g := range game.SupportedGames.Iter() {
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
		"assumeQueue":            "Automatch.AssumeQueue",
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
	if err := migrateHosts(usedFile, &c); err != nil {
		logger.Fail("%s", err)
		os.Exit(common.ErrConfigParse)
	}
	return &c, usedFile
}

// migrateHosts moves the old Games.<game>.Hosts lists to the global [Hosts]
// section. A silent union would change the binding model, so differing lists
// stop the boot with the migration instruction instead.
func migrateHosts(usedFile string, c *internal.Configuration) error {
	fileHasHosts := false
	if usedFile != "" {
		fk := koanf.New(".")
		if err := fk.Load(file.Provider(usedFile), toml.Parser()); err == nil {
			fileHasHosts = fk.Exists("Hosts")
		}
	}
	if fileHasHosts {
		for _, gameID := range c.Games.Enabled {
			if g := c.Games.Get(gameID); g != nil && len(g.Hosts) > 0 {
				logger.Warn("%s: [Games.%s.Hosts] is deprecated; use [Hosts] Modern/Legacy", usedFile, gameID)
			}
		}
		return nil
	}
	var migrated []string
	var firstGame string
	for _, gameID := range c.Games.Enabled {
		g := c.Games.Get(gameID)
		if g == nil || len(g.Hosts) == 0 {
			continue
		}
		if firstGame == "" {
			firstGame = gameID
			migrated = slices.Clone(g.Hosts)
			continue
		}
		if !slices.Equal(migrated, g.Hosts) {
			return fmt.Errorf("Games.%s.Hosts %v and Games.%s.Hosts %v differ; hosts are global now: remove them and set [Hosts] Modern/Legacy", firstGame, migrated, gameID, g.Hosts)
		}
	}
	if firstGame != "" {
		// The destination is the certificate class of the enabled set: a v2.0
		// document only serves age1-3, whose clients may reject the CA-signed
		// leaf, so migrating them to Modern would break them. Moving to one
		// class empties the other: the default would otherwise keep listening
		// on both.
		allLegacy, allModern := true, true
		for _, gameID := range c.Games.Enabled {
			if common.GameCertMode(gameID) == common.CertModeLegacy {
				allModern = false
			} else {
				allLegacy = false
			}
		}
		switch {
		case allLegacy:
			c.Hosts.Legacy = migrated
			c.Hosts.Modern = nil
			logger.Warn("Games.<game>.Hosts is deprecated; migrated to [Hosts] Legacy (every enabled game is legacy class)")
		case allModern:
			c.Hosts.Modern = migrated
			c.Hosts.Legacy = nil
			logger.Warn("Games.<game>.Hosts is deprecated; migrated to [Hosts] Modern (every enabled game is modern class)")
		default:
			c.Hosts.Modern = migrated
			c.Hosts.Legacy = nil
			logger.Warn("Games.<game>.Hosts is deprecated; migrated to [Hosts] Modern; age1-3 acceptance of CA-signed certificates depends on the M0 spike")
		}
	}
	return nil
}

// sizeLimits are the parsed MaxFileSize/MaxTotalSize/Expiry triple shared by the
// CustomFiles and Replays sections.
type sizeLimits struct {
	maxFileSize  int64
	maxTotalSize int64
	expiry       time.Duration
}

// resolveSizeLimits parses and cross-checks the MaxFileSize/MaxTotalSize/Expiry
// triple shared by the CustomFiles and Replays sections. Sizes accept the usual
// B/KB/MB/GB suffixes; the TTL is a Go duration such as 12h or 5m. Expiry <= 0 is
// left to the caller: CustomFiles rejects it, Replays reads it as "keep forever".
func resolveSizeLimits(maxFileSizeRaw, maxTotalSizeRaw, expiryRaw string) (sizeLimits, error) {
	maxFileSize, err := bytesize.Parse(maxFileSizeRaw)
	if err != nil {
		return sizeLimits{}, fmt.Errorf("MaxFileSize: %w", err)
	}
	maxTotalSize, err := bytesize.Parse(maxTotalSizeRaw)
	if err != nil {
		return sizeLimits{}, fmt.Errorf("MaxTotalSize: %w", err)
	}
	if maxFileSize <= 0 {
		return sizeLimits{}, fmt.Errorf("MaxFileSize must be greater than zero")
	}
	if maxTotalSize <= 0 {
		return sizeLimits{}, fmt.Errorf("MaxTotalSize must be greater than zero")
	}
	if maxFileSize > maxTotalSize {
		return sizeLimits{}, fmt.Errorf("MaxFileSize (%d) must not exceed MaxTotalSize (%d)", maxFileSize, maxTotalSize)
	}
	expiry, err := time.ParseDuration(strings.TrimSpace(expiryRaw))
	if err != nil {
		return sizeLimits{}, fmt.Errorf("expiry: %w", err)
	}
	return sizeLimits{maxFileSize: maxFileSize, maxTotalSize: maxTotalSize, expiry: expiry}, nil
}

// applyCustomFilesConfig validates the CustomFiles section and publishes the
// resolved values. The embedded ageAPIServer library serves the endpoints now,
// so the values are handed to it when it is constructed.
func applyCustomFilesConfig(cfg *internal.Configuration) error {
	// The default lives under the server's resources directory, as the mods
	// paths do. 'auto' and an empty value resolve there too, so documents
	// written before the default was explicit keep working.
	folder := cfg.CustomFiles.Folder
	if folder == "" || folder == "auto" {
		folder = filepath.Join(paths.ResourcesDir, "custom-files")
	}
	internal.CustomFilesConfig = internal.CustomFilesConfiguration{
		Enabled: cfg.CustomFiles.Enabled,
		Folder:  folder,
	}
	if !cfg.CustomFiles.Enabled {
		// Nothing reads the limits or the TTL while the endpoints are off, so a
		// typo in them must not stop a server that does not use the feature.
		return nil
	}
	limits, err := resolveSizeLimits(cfg.CustomFiles.MaxFileSize, cfg.CustomFiles.MaxTotalSize, cfg.CustomFiles.Expiry)
	if err != nil {
		return err
	}
	if limits.expiry <= 0 {
		return fmt.Errorf("expiry must be greater than zero")
	}
	internal.CustomFilesConfig.MaxFileSize = limits.maxFileSize
	internal.CustomFilesConfig.MaxTotalSize = limits.maxTotalSize
	internal.CustomFilesConfig.Expiry = limits.expiry
	return nil
}

// applyReplaysConfig validates the Replays section and publishes the resolved
// values in the internal runtime globals.
func applyReplaysConfig(cfg *internal.Configuration) error {
	internal.ReplaysEnabled = cfg.Replays.Enabled
	if !cfg.Replays.Enabled {
		return nil
	}
	limits, err := resolveSizeLimits(cfg.Replays.MaxFileSize, cfg.Replays.MaxTotalSize, cfg.Replays.Expiry)
	if err != nil {
		return err
	}
	// Expiry <= 0 means keep forever (no TTL-based expiration)
	internal.ReplaysMaxFileSize = limits.maxFileSize
	internal.ReplaysMaxTotalSize = limits.maxTotalSize
	internal.ReplaysExpiry = limits.expiry
	return nil
}
