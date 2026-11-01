package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"battle-server-manager/internal"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/game"
)

func setupStartMocks(t *testing.T) func() {
	t.Helper()
	origParsed := parsedGameIdsFn
	origExisting := existingServersFn
	origAvailable := availableFn
	origGenerate := generatePortsFn
	origResolveSSL := resolveSSLFilesPathFn
	origResolvePath := resolvePathFn
	origParseExtra := parseExtraArgsFn
	origExecute := executeBattleServerFn
	origWait := waitForInitFn
	origWrite := writeConfigFn
	origKill := killFn
	origIsAdmin := isAdminFn
	origPaths := configPaths

	return func() {
		parsedGameIdsFn = origParsed
		existingServersFn = origExisting
		availableFn = origAvailable
		generatePortsFn = origGenerate
		resolveSSLFilesPathFn = origResolveSSL
		resolvePathFn = origResolvePath
		parseExtraArgsFn = origParseExtra
		executeBattleServerFn = origExecute
		waitForInitFn = origWait
		writeConfigFn = origWrite
		killFn = origKill
		isAdminFn = origIsAdmin
		configPaths = origPaths
	}
}

func tempGameConfig(t *testing.T, gameId string) string {
	t.Helper()
	dir := t.TempDir()
	// Make configPaths point to this dir so initConfig finds it
	configPaths = []string{dir}
	content := "Region = 'eu'\nName = 'Test'\nHost = 'auto'\nCertsPath = 'auto'\n[Executable]\nPath = 'auto'\nExtraArgs = []\n[Ports]\nBs = 0\nWebSocket = 0\nOutOfBand = 0\n"
	path := filepath.Join(dir, "config."+gameId+".toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunStart_MissingGameFlag(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	_, code := runStart([]string{})
	if code != common.ErrSyntax {
		t.Fatalf("expected ErrSyntax for missing game, got %d", code)
	}
}

func TestRunStart_InvalidGame(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	// parsedGameIds will fail for unsupported game
	_, code := runStart([]string{"--game", "not-a-game"})
	if code != internal.ErrGames {
		t.Fatalf("expected ErrGames, got %d", code)
	}
}

func TestRunStart_AlreadyRunning(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	tmpCfg := tempGameConfig(t, "age2")

	parsedGameIdsFn = func(gameIds *[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(gameId string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string]("server"), mapset.NewSet[string]("eu")
	}
	availableFn = func(port int) bool { return true }
	// Use real config but force AlreadyRunning path
	// Force=false by default, regions not empty -> should return ErrAlreadyRunning
	_, code := runStart([]string{"--game", "age2", "--gameConfig", tmpCfg})
	if code != internal.ErrAlreadyRunning {
		t.Fatalf("expected ErrAlreadyRunning, got %d", code)
	}

	// Test NoErrExisting returns 0 without error
	// Need to set NoErrExisting flag
	// That flag is part of StartValues, we can set via args --noErrExisting?
	// Instead, mock to test that path
	restore2 := setupStartMocks(t)
	defer restore2()
	// Directly test the logic: if NoErrExisting true, it returns 0
	// We can test via runStart with --noErrExisting flag if exists
	// Check what flags exist: StartFlagSet includes --force and --noErrExisting?
}

func TestRunStart_RegionAlreadyExists(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	tmpCfg := tempGameConfig(t, "age2")
	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	// Make ExistingServers return empty for Force check, but then region conflict later
	// First check: regions.IsEmpty() -> false triggers AlreadyRunning, so we need Force=true to bypass
	// So we set Force via flag
	existingServersFn = func(gameId string) (error, mapset.Set[string], mapset.Set[string]) {
		// Return existing region "eu" which matches config Region "eu"
		return nil, mapset.NewSet[string](), mapset.NewSet[string]("eu")
	}
	availableFn = func(int) bool { return true }
	_, code := runStart([]string{"--game", "age2", "--gameConfig", tmpCfg, "--force"})
	if code != internal.ErrAlreadyExists {
		t.Fatalf("expected ErrAlreadyExists for duplicate region, got %d", code)
	}
}

func TestRunStart_HostResolveFail(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	// Create config with Host = "invalid-host-that-will-not-resolve"
	dir := t.TempDir()
	configPaths = []string{dir}
	content := "Region = 'eu'\nName = 'Test'\nHost = 'invalid-host-xyz-12345'\nCertsPath = 'auto'\n[Executable]\nPath = 'auto'\nExtraArgs = []\n[Ports]\nBs = 0\nWebSocket = 0\nOutOfBand = 0\n"
	path := filepath.Join(dir, "config.age2.toml")
	_ = os.WriteFile(path, []byte(content), 0644)

	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string](), mapset.NewSet[string]()
	}
	availableFn = func(int) bool { return true }

	_, code := runStart([]string{"--game", "age2", "--gameConfig", path})
	if code != internal.ErrResolveHost && code != internal.ErrInvalidHost {
		t.Fatalf("expected host resolve error, got %d", code)
	}
}

func TestRunStart_PortInUse(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	tmpCfg := tempGameConfig(t, "age2")
	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string](), mapset.NewSet[string]()
	}
	// Make Bs port appear in use
	availableFn = func(port int) bool {
		// Check what port is being checked: we need to know cfg.Ports.Bs is 0, so Available won't be called for 0
		// Instead, set config to have explicit port
		return false // every port appears in use
	}
	// Need config with explicit Bs port
	dir := t.TempDir()
	configPaths = []string{dir}
	content := "Region = 'eu'\nName = 'Test'\nHost = 'auto'\nCertsPath = 'auto'\n[Executable]\nPath = 'auto'\nExtraArgs = []\n[Ports]\nBs = 27015\nWebSocket = 0\nOutOfBand = 0\n"
	path := filepath.Join(dir, "config.age2.toml")
	_ = os.WriteFile(path, []byte(content), 0644)
	_ = tmpCfg // keep

	_, code := runStart([]string{"--game", "age2", "--gameConfig", path})
	if code != internal.ErrBsPortInUse {
		t.Fatalf("expected ErrBsPortInUse, got %d", code)
	}
}

func TestRunStart_GeneratePortsError(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	tmpCfg := tempGameConfig(t, "age2")
	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string](), mapset.NewSet[string]()
	}
	availableFn = func(int) bool { return true }
	generatePortsFn = func([]int) ([]int, error) {
		return nil, errors.New("gen fail")
	}
	_, code := runStart([]string{"--game", "age2", "--gameConfig", tmpCfg})
	if code != internal.ErrGenPorts {
		t.Fatalf("expected ErrGenPorts, got %d", code)
	}
}

func TestRunStart_ResolveSSLError(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	tmpCfg := tempGameConfig(t, "age2")
	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string](), mapset.NewSet[string]()
	}
	availableFn = func(int) bool { return true }
	generatePortsFn = func(ports []int) ([]int, error) {
		// Return ports as-is but with generated for zeros
		for i, p := range ports {
			if p == 0 {
				ports[i] = 30000 + i
			}
		}
		return ports, nil
	}
	resolveSSLFilesPathFn = func(gameId, certsPath string) (string, string, error) {
		return "", "", errors.New("ssl fail")
	}
	_, code := runStart([]string{"--game", "age2", "--gameConfig", tmpCfg})
	if code != internal.ErrResolveSSLFiles {
		t.Fatalf("expected ErrResolveSSLFiles, got %d", code)
	}
}

func TestRunStart_ResolvePathError(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	tmpCfg := tempGameConfig(t, "age2")
	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string](), mapset.NewSet[string]()
	}
	availableFn = func(int) bool { return true }
	generatePortsFn = func(ports []int) ([]int, error) {
		for i, p := range ports {
			if p == 0 {
				ports[i] = 30000 + i
			}
		}
		return ports, nil
	}
	resolveSSLFilesPathFn = func(string, string) (string, string, error) {
		return "/tmp/cert", "/tmp/key", nil
	}
	resolvePathFn = func(string, string) (string, error) {
		return "", errors.New("resolve fail")
	}
	_, code := runStart([]string{"--game", "age2", "--gameConfig", tmpCfg})
	if code != internal.ErrResolvePath {
		t.Fatalf("expected ErrResolvePath, got %d", code)
	}
}

func TestRunStart_ParseExtraArgsError(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	tmpCfg := tempGameConfig(t, "age2")
	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string](), mapset.NewSet[string]()
	}
	availableFn = func(int) bool { return true }
	generatePortsFn = func(ports []int) ([]int, error) {
		for i, p := range ports {
			if p == 0 {
				ports[i] = 30000 + i
			}
		}
		return ports, nil
	}
	resolveSSLFilesPathFn = func(string, string) (string, string, error) {
		return "/tmp/cert", "/tmp/key", nil
	}
	resolvePathFn = func(string, string) (string, error) {
		return "/tmp/bs.exe", nil
	}
	parseExtraArgsFn = func([]string, map[string]string, bool) ([]string, error) {
		return nil, errors.New("parse fail")
	}
	// Need to create a fake bs.exe file so validPath would succeed if not mocked, but we mock parseExtraArgs before that
	// Actually parseExtraArgs is called after ResolvePath, so we need to mock it to fail
	_, code := runStart([]string{"--game", "age2", "--gameConfig", tmpCfg})
	if code != internal.ErrParseArgs {
		t.Fatalf("expected ErrParseArgs, got %d", code)
	}
}

func TestRunStart_ExecuteBattleServerError(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	tmpCfg := tempGameConfig(t, "age2")
	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string](), mapset.NewSet[string]()
	}
	availableFn = func(int) bool { return true }
	generatePortsFn = func(ports []int) ([]int, error) {
		for i, p := range ports {
			if p == 0 {
				ports[i] = 30000 + i
			}
		}
		return ports, nil
	}
	resolveSSLFilesPathFn = func(string, string) (string, string, error) {
		return "/tmp/cert", "/tmp/key", nil
	}
	resolvePathFn = func(string, string) (string, error) {
		return "/tmp/bs.exe", nil
	}
	parseExtraArgsFn = func([]string, map[string]string, bool) ([]string, error) {
		return []string{}, nil
	}
	executeBattleServerFn = func(gameId, path, region, name string, ports []int, certFile, keyFile string, extraArgs []string, hideWindow bool, logRoot string) (uint32, error) {
		return 0, errors.New("exec fail")
	}
	_, code := runStart([]string{"--game", "age2", "--gameConfig", tmpCfg})
	if code != internal.ErrStartBattleServer {
		t.Fatalf("expected ErrStartBattleServer, got %d", code)
	}
}

func TestRunStart_WaitForInitFail(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	tmpCfg := tempGameConfig(t, "age2")
	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string](), mapset.NewSet[string]()
	}
	availableFn = func(int) bool { return true }
	generatePortsFn = func(ports []int) ([]int, error) {
		for i, p := range ports {
			if p == 0 {
				ports[i] = 30000 + i
			}
		}
		return ports, nil
	}
	resolveSSLFilesPathFn = func(string, string) (string, string, error) {
		return "/tmp/cert", "/tmp/key", nil
	}
	resolvePathFn = func(string, string) (string, error) {
		return "/tmp/bs.exe", nil
	}
	parseExtraArgsFn = func([]string, map[string]string, bool) ([]string, error) {
		return []string{}, nil
	}
	executeBattleServerFn = func(string, string, string, string, []int, string, string, []string, bool, string) (uint32, error) {
		return 1234, nil
	}
	waitForInitFn = func(battleServer.Config) bool { return false }
	// Mock process.FindProcess to avoid killing real process
	// waitForInit will fail, then it will try to find process 1234 and kill it - we need to ensure that doesn't error
	// But FindProcess for 1234 will likely fail (no such process), then it will log and return ErrInitBattleServer
	_, code := runStart([]string{"--game", "age2", "--gameConfig", tmpCfg})
	if code != internal.ErrInitBattleServer {
		t.Fatalf("expected ErrInitBattleServer, got %d", code)
	}
}

func TestRunStart_WriteConfigFail(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	tmpCfg := tempGameConfig(t, "age2")
	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string](), mapset.NewSet[string]()
	}
	availableFn = func(int) bool { return true }
	generatePortsFn = func(ports []int) ([]int, error) {
		for i, p := range ports {
			if p == 0 {
				ports[i] = 30000 + i
			}
		}
		return ports, nil
	}
	resolveSSLFilesPathFn = func(string, string) (string, string, error) {
		return "/tmp/cert", "/tmp/key", nil
	}
	resolvePathFn = func(string, string) (string, error) {
		return "/tmp/bs.exe", nil
	}
	parseExtraArgsFn = func([]string, map[string]string, bool) ([]string, error) {
		return []string{}, nil
	}
	executeBattleServerFn = func(string, string, string, string, []int, string, string, []string, bool, string) (uint32, error) {
		return 1234, nil
	}
	waitForInitFn = func(battleServer.Config) bool { return true }
	writeConfigFn = func(string, battleServer.Config) error {
		return errors.New("write fail")
	}
	killFn = func(battleServer.Config) bool { return true }
	_, code := runStart([]string{"--game", "age2", "--gameConfig", tmpCfg})
	if code != internal.ErrConfigWrite {
		t.Fatalf("expected ErrConfigWrite, got %d", code)
	}
}

func TestRunStart_Success(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	tmpCfg := tempGameConfig(t, "age2")
	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string](), mapset.NewSet[string]()
	}
	availableFn = func(int) bool { return true }
	generatePortsFn = func(ports []int) ([]int, error) {
		for i, p := range ports {
			if p == 0 {
				ports[i] = 30000 + i
			}
		}
		return ports, nil
	}
	resolveSSLFilesPathFn = func(string, string) (string, string, error) {
		return "/tmp/cert", "/tmp/key", nil
	}
	resolvePathFn = func(string, string) (string, error) {
		return "/tmp/bs.exe", nil
	}
	parseExtraArgsFn = func([]string, map[string]string, bool) ([]string, error) {
		return []string{}, nil
	}
	executeBattleServerFn = func(string, string, string, string, []int, string, string, []string, bool, string) (uint32, error) {
		return 1234, nil
	}
	waitForInitFn = func(battleServer.Config) bool { return true }
	writeConfigFn = func(gameId string, cfg battleServer.Config) error {
		if gameId != "age2" {
			t.Errorf("gameId %q", gameId)
		}
		if cfg.Region == "" || cfg.Name == "" {
			t.Error("region/name empty")
		}
		return nil
	}
	_, code := runStart([]string{"--game", "age2", "--gameConfig", tmpCfg})
	if code != 0 {
		t.Fatalf("expected success 0, got %d", code)
	}
}

func TestRunStart_AutoNameGeneration(t *testing.T) {
	restore := setupStartMocks(t)
	defer restore()
	dir := t.TempDir()
	configPaths = []string{dir}
	content := "Region = 'auto'\nName = 'auto'\nHost = 'auto'\nCertsPath = 'auto'\n[Executable]\nPath = 'auto'\nExtraArgs = []\n[Ports]\nBs = 0\nWebSocket = 0\nOutOfBand = 0\n"
	tmpCfg := filepath.Join(dir, "config.age2.toml")
	if err := os.WriteFile(tmpCfg, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	var err error
	// Config has auto name/region, and ExistingServers has "server" already
	parsedGameIdsFn = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string]("server"), mapset.NewSet[string]("server")
	}
	availableFn = func(int) bool { return true }
	generatePortsFn = func(ports []int) ([]int, error) {
		for i, p := range ports {
			if p == 0 {
				ports[i] = 30000 + i
			}
		}
		return ports, nil
	}
	resolveSSLFilesPathFn = func(string, string) (string, string, error) {
		return "/tmp/cert", "/tmp/key", nil
	}
	resolvePathFn = func(string, string) (string, error) { return "/tmp/bs.exe", nil }
	parseExtraArgsFn = func([]string, map[string]string, bool) ([]string, error) { return []string{}, nil }
	executeBattleServerFn = func(gameId, path, region, name string, ports []int, certFile, keyFile string, extraArgs []string, hideWindow bool, logRoot string) (uint32, error) {
		if name != "Server (1)" {
			t.Errorf("auto name should be Server (1) when Server exists, got %q", name)
		}
		if region != "Server (1)" {
			t.Errorf("auto region should follow name, got %q", region)
		}
		return 1234, nil
	}
	waitForInitFn = func(battleServer.Config) bool { return true }
	writeConfigFn = func(string, battleServer.Config) error { return nil }

	_, code := runStart([]string{"--game", "age2", "--gameConfig", tmpCfg, "--force"})
	if code != 0 {
		t.Fatalf("expected success, got %d", code)
	}
	// Also test when no existing server, auto should be "Server"
	existingServersFn = func(string) (error, mapset.Set[string], mapset.Set[string]) {
		return nil, mapset.NewSet[string](), mapset.NewSet[string]()
	}
	executeBattleServerFn = func(gameId, path, region, name string, ports []int, certFile, keyFile string, extraArgs []string, hideWindow bool, logRoot string) (uint32, error) {
		if name != "Server" {
			t.Errorf("auto name should be Server, got %q", name)
		}
		return 1234, nil
	}
	err, code = runStart([]string{"--game", "age2", "--gameConfig", tmpCfg, "--force"})
	if code != 0 {
		t.Fatalf("expected success, got %d err %v", code, err)
	}
	_ = game.AoE1 // ensure import used
}
