package cmd

import (
	"battle-server-manager/internal"
	"battle-server-manager/internal/cmdUtils"
	"battle-server-manager/internal/cmdUtils/executor"
	"battle-server-manager/internal/cmdUtils/resolver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/v2"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/cmd/bsManager"
	"github.com/luskaner/ageServer/common/cmdlog"
	"github.com/luskaner/ageServer/common/executables"
	commonExecutor "github.com/luskaner/ageServer/common/executor"
	"github.com/luskaner/ageServer/common/game"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/common/paths"
	"github.com/luskaner/ageServer/common/process"
	"github.com/spf13/pflag"
)

var configPaths = []string{paths.ResourcesDir, "."}

var (
	parsedGameIdsFn       = cmdUtils.ParsedGameIds
	existingServersFn     = cmdUtils.ExistingServers
	availableFn           = cmdUtils.Available
	generatePortsFn       = cmdUtils.GeneratePortsAsNeeded
	resolveSSLFilesPathFn = cmdUtils.ResolveSSLFilesPath
	resolvePathFn         = resolver.ResolvePath
	parseExtraArgsFn      = common.ParseCommandArgsFromSlice
	executeBattleServerFn = executor.ExecuteBattleServer
	waitForInitFn         = cmdUtils.WaitForBattleServerInit
	writeConfigFn         = cmdUtils.WriteConfig
	killFn                = cmdUtils.Kill
	isAdminFn             = commonExecutor.IsAdmin
)

func runStart(args []string) (err error, exitCode int) {
	values, flags := bsManager.StartFlagSet(configPaths)
	if err = flags.Parse(args); err != nil {
		exitCode = common.ErrSyntax
		return
	}
	// validate required flags
	if values.GameId == "" {
		return errors.New("required flag 'game' not set"), common.ErrSyntax
	}

	cfg := initConfig(flags, values)
	gameIds := []string{values.GameId}
	var games mapset.Set[string]
	games, err = parsedGameIdsFn(&gameIds)
	if err != nil {
		cmdlog.Fail("%s", err.Error())
		exitCode = internal.ErrGames
		return
	}
	values.GameId, _ = games.Pop()
	cmdlog.Step("Checking and resolving configuration...")
	isAdmin := isAdminFn()
	if isAdmin {
		cmdlog.Warn("Running as administrator, this is not needed and might cause issues.")
	}
	name := cfg.Name
	region := cfg.Region
	var names mapset.Set[string]
	var regions mapset.Set[string]
	err, names, regions = existingServersFn(values.GameId)
	if err != nil {
		cmdlog.Fail("Could not get existing servers: %s", err.Error())
		exitCode = internal.ErrReadConfig
		return
	}
	if !values.Force && !regions.IsEmpty() {
		if values.NoErrExisting {
			return
		}
		cmdlog.Fail("A standalone Battle Server is already running, use --force to start another one")
		exitCode = internal.ErrAlreadyRunning
		return
	}
	if name == "auto" || region == "auto" {
		if name == "auto" {
			if names.ContainsOne("server") || regions.ContainsOne("server") {
				for i := 1; ; i++ {
					if currentName := fmt.Sprintf("Server (%d)", i); !names.ContainsOne(currentName) && !regions.ContainsOne(currentName) {
						name = currentName
						break
					}
				}
			} else {
				name = "Server"
			}
			cmdlog.Info("Auto-generated name: %s", name)
		}
		if region == "auto" {
			region = name
			cmdlog.Info("Auto-generated region: %s", region)
		}
	}
	if lowerRegion := strings.ToLower(region); names.ContainsOne(lowerRegion) || regions.ContainsOne(lowerRegion) {
		cmdlog.Fail("A standalone Battle Server with the name/region %s already exists", region)
		exitCode = internal.ErrAlreadyExists
		return
	}
	if lowerName := strings.ToLower(name); names.ContainsOne(lowerName) || regions.ContainsOne(lowerName) {
		cmdlog.Fail("A standalone Battle Server with the name/region %s already exists", name)
		exitCode = internal.ErrAlreadyExists
		return
	}
	host := cfg.Host
	var ip string
	if host != "auto" {
		ips := common.HostOrIpToIps(host)
		if len(ips) == 0 {
			cmdlog.Fail("Could not resolve host to an IP address")
			exitCode = internal.ErrResolveHost
			return
		}
		ip = selectNonLoopbackIP(ips)
		if ip == "" {
			cmdlog.Fail("IP not valid or could not resolve host to a suitable IP address")
			exitCode = internal.ErrInvalidHost
			return
		}
		if ip != host {
			cmdlog.Info("Resolved host to IP address: %s", ip)
		}
	} else {
		ip = host
	}
	bsPort := cfg.Ports.Bs
	websocketPort := cfg.Ports.WebSocket
	outOfBandPort := -1
	if values.GameId != game.AoE1 {
		outOfBandPort = cfg.Ports.OutOfBand
	}
	if bsPort > 0 && !availableFn(bsPort) {
		cmdlog.Fail("Bs port %d is already in use", bsPort)
		exitCode = internal.ErrBsPortInUse
		return
	}
	if websocketPort > 0 && !availableFn(websocketPort) {
		cmdlog.Fail("WebSocket port %d is already in use", websocketPort)
		exitCode = internal.ErrWsPortInUse
		return
	}
	if outOfBandPort > 0 && !availableFn(outOfBandPort) {
		cmdlog.Fail("Out of band port %d is already in use", outOfBandPort)
		exitCode = internal.ErrOobPortInUse
		return
	}
	allPorts, err := generatePortsFn([]int{bsPort, websocketPort, outOfBandPort})
	if err != nil {
		cmdlog.Fail("Could not generate ports: %s", err)
		exitCode = internal.ErrGenPorts
		return
	}
	if bsPort != allPorts[0] {
		cmdlog.Info("Auto-generated BsPort port: %d", allPorts[0])
	}
	if websocketPort != allPorts[1] {
		cmdlog.Info("Auto-generated WebSocketPort port: %d", allPorts[1])
	}
	if outOfBandPort != allPorts[2] {
		cmdlog.Info("Auto-generated Out Of Band Port: %d", allPorts[2])
	}
	resolvedCertFile, resolvedKeyFile, err := resolveSSLFilesPathFn(
		values.GameId,
		cfg.CertsPath,
	)
	if err != nil {
		cmdlog.Fail("Could not resolve SSL files: %s", err)
		exitCode = internal.ErrResolveSSLFiles
		return
	}
	resolvedPath, err := resolvePathFn(values.GameId, cfg.Executable.Path)
	if err != nil {
		cmdlog.Fail("Could not resolve path: %s", err)
		exitCode = internal.ErrResolvePath
		return
	}
	extraArgs, err := parseExtraArgsFn(cfg.Executable.ExtraArgs, nil, true)
	if err != nil {
		cmdlog.Fail("Could not parse extra args: %s", err)
		exitCode = internal.ErrParseArgs
		return
	}
	var pid uint32
	pid, err = executeBattleServerFn(
		values.GameId,
		resolvedPath,
		region,
		name,
		allPorts,
		resolvedCertFile,
		resolvedKeyFile,
		extraArgs,
		values.HideWindow,
		values.LogRoot,
	)
	if err != nil {
		cmdlog.Fail("Could not execute BattleServer: %s", err)
		exitCode = internal.ErrStartBattleServer
		return
	}
	saveConfig := battleServer.Config{
		Region:        region,
		Name:          name,
		IPv4:          ip,
		BsPort:        allPorts[0],
		WebSocketPort: allPorts[1],
		PID:           pid,
	}
	if allPorts[2] != -1 {
		saveConfig.OutOfBandPort = allPorts[2]
	}
	if !waitForInitFn(saveConfig) {
		cmdlog.Fail("Battle Server initialization did not complete in time")
		if proc, localErr := process.FindProcess(int(saveConfig.PID)); localErr == nil && proc != nil {
			if localErr := process.KillProc(proc); localErr != nil {
				cmdlog.Fail("Error: %s", localErr)
			} else {
				cmdlog.Ok("OK.")
			}
		} else if localErr != nil {
			cmdlog.Warn("Could not find the process to kill: %s", localErr)
		}
		exitCode = internal.ErrInitBattleServer
		return
	}
	if err = writeConfigFn(values.GameId, saveConfig); err != nil {
		cmdlog.Fail("Could not write config: %s", err)
		cmdlog.Step("Stopping started Battle Server...")
		killFn(saveConfig)
		exitCode = internal.ErrConfigWrite
	}
	return
}

func initConfig(fs *pflag.FlagSet, values *bsManager.StartValues) *internal.Configuration {
	k := koanf.New(".")
	defaults := map[string]any{
		"Region":               "auto",
		"Name":                 "auto",
		"Host":                 "auto",
		"CertsPath":            "auto",
		"Executable.Path":      "auto",
		"Executable.ExtraArgs": []string{},
		"Ports.Bs":             0,
		"Ports.WebSocket":      0,
		"Ports.OutOfBand":      0,
	}

	var fileCandidates []string
	if values.GameCfgFile != "" {
		fileCandidates = append(fileCandidates, values.GameCfgFile)
	} else {
		for _, configPath := range configPaths {
			fileCandidates = append(fileCandidates, filepath.Join(configPath, fmt.Sprintf("config.%s.toml", values.GameId)))
		}
	}

	usedFile := common.LoadKoanfLayersOrExit(k, defaults, fileCandidates, toml.Parser(), fs, nil, executables.BattleServerManager, commonLogger.Println)
	if values.GameCfgFile != "" && usedFile == "" {
		cmdlog.Warn("No config file found, using defaults.")
	}
	if usedFile != "" {
		cmdlog.Info("Using config file: %s", usedFile)
		if values.LogRoot != "" {
			data, err := os.ReadFile(usedFile)
			if err != nil {
				cmdlog.Warn("Could not read config file for logging: %s", err)
			} else {
				commonLogger.PrefixPrintln("config", string(data))
			}
		}
	}

	var c internal.Configuration
	if err := k.Unmarshal("", &c); err != nil {
		cmdlog.Fail("Unable to decode configuration: %s", err)
		os.Exit(common.ErrConfigParse)
	}
	return &c
}
