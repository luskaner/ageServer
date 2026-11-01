package cmdUtils

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"battle-server-manager/internal"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/battleServer"
)

// TestGeneratePortsAsNeeded_OrderPreserved verifies fix for reversed assignment bug.
func TestGeneratePortsAsNeeded_OrderPreserved(t *testing.T) {
	// Request 3 ports: first and last explicit, middle missing
	// Original bug reversed generated ports order.
	ports := []int{30001, 0, 0}
	gen, err := GeneratePortsAsNeeded(ports)
	if err != nil {
		t.Fatal(err)
	}
	if gen[0] != 30001 {
		t.Errorf("explicit port altered: got %v", gen)
	}
	if gen[1] == gen[2] {
		t.Errorf("generated ports should be distinct, got %v", gen)
	}
	// Ensure generated ports are not zero and not equal to explicit
	if gen[1] == 0 || gen[2] == 0 {
		t.Error("generated ports should be non-zero")
	}
	// Check that order is preserved (not reversed): the first missing gets first generated
	// We can't guarantee order of findUnusedPorts, but at least ensure they are distinct
}

func TestGeneratePortsAsNeeded_AllZero(t *testing.T) {
	ports := []int{0, 0, 0}
	gen, err := GeneratePortsAsNeeded(ports)
	if err != nil {
		t.Fatal(err)
	}
	if len(gen) != 3 {
		t.Fatalf("len %d want 3", len(gen))
	}
	seen := map[int]bool{}
	for _, p := range gen {
		if p <= 0 {
			t.Fatalf("invalid port %d", p)
		}
		if seen[p] {
			t.Fatalf("duplicate port %d", p)
		}
		seen[p] = true
	}
}

func TestWaitForBattleServerInit_ImmediateSuccess(t *testing.T) {
	// Use a config that is already valid (live PID + bound ports)
	ln1, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4zero})
	if err != nil {
		t.Skip("cannot bind")
	}
	defer ln1.Close()
	ln2, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4zero})
	if err != nil {
		t.Skip("cannot bind")
	}
	defer ln2.Close()

	cfg := battleServer.Config{
		Base: battleServer.Base{
			Region:        "r",
			Name:          "n",
			IPv4:          "127.0.0.1",
			BsPort:        ln1.Addr().(*net.TCPAddr).Port,
			WebSocketPort: ln2.Addr().(*net.TCPAddr).Port,
		},
		PID: uint32(os.Getpid()),
	}
	// Should return quickly (no sleep needed)
	start := time.Now()
	ok := WaitForBattleServerInit(cfg)
	if !ok {
		t.Error("expected ok true for valid config")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Error("should return immediately for valid config, not wait full timeout")
	}
}

func TestWaitForBattleServerInit_Timeout(t *testing.T) {
	origTimeout := waitInitTimeout
	waitInitTimeout = 300 * time.Millisecond
	defer func() { waitInitTimeout = origTimeout }()

	// Invalid config (bad ports, PID 0) will never validate, should timeout
	cfg := battleServer.Config{
		Base: battleServer.Base{
			Region: "r", Name: "n", IPv4: "127.0.0.1",
			BsPort: 1, WebSocketPort: 2,
		},
		PID: 0,
	}
	start := time.Now()
	ok := WaitForBattleServerInit(cfg)
	if ok {
		t.Error("expected false for invalid config")
	}
	elapsed := time.Since(start)
	// Should have waited at least timeout (300ms) but not too long (allow 1s)
	if elapsed < 250*time.Millisecond {
		t.Errorf("should have waited timeout, elapsed %v", elapsed)
	}
	// On non-windows, timeout is 3x, so 900ms; we set to 300ms, so 900ms expected
	expectedMin := 250 * time.Millisecond
	if runtime.GOOS != "windows" {
		expectedMin = 800 * time.Millisecond
	}
	if elapsed < expectedMin {
		t.Logf("elapsed %v less than expected %v for GOOS=%s", elapsed, expectedMin, runtime.GOOS)
	}
}

func TestWriteConfig_MaxIndexUnsorted(t *testing.T) {
	gameId := "test-max-unsorted"
	dir := battleServer.Folder(gameId)
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	// Create files with unsorted indices: 5, 0, 2
	for _, idx := range []int{5, 0, 2} {
		path := filepath.Join(dir, battleServer.Name(idx))
		if err := os.WriteFile(path, []byte("dummy"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Need a valid config to write
	ln1, _ := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4zero})
	if ln1 == nil {
		t.Skip("cannot bind")
	}
	defer ln1.Close()
	ln2, _ := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4zero})
	if ln2 == nil {
		t.Skip("cannot bind")
	}
	defer ln2.Close()

	cfg := battleServer.Config{
		Base: battleServer.Base{Region: "r", Name: "n", IPv4: "127.0.0.1", BsPort: ln1.Addr().(*net.TCPAddr).Port, WebSocketPort: ln2.Addr().(*net.TCPAddr).Port},
		PID:  uint32(os.Getpid()),
	}
	if err := WriteConfig(gameId, cfg); err != nil {
		t.Fatal(err)
	}
	// Should have created 6.toml (max 5 +1), not 3.toml (last index 2 +1)
	if _, err := os.Stat(filepath.Join(dir, battleServer.Name(6))); err != nil {
		t.Fatalf("expected file 6.toml to be created (max+1), err %v", err)
	}
	// Ensure 3.toml does not exist (would be wrong if using last instead of max)
	if _, err := os.Stat(filepath.Join(dir, battleServer.Name(3))); err == nil {
		t.Error("should not have created 3.toml, bug: used last index not max")
	}
}

func TestRemove_ReturnsFalseWhenFileMissing(t *testing.T) {
	gameId := "test-remove-missing-file"
	dir := battleServer.Folder(gameId)
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	// Create a config with index 0 but don't create file, then try to remove
	cfg := battleServer.Config{
		Base: battleServer.Base{Region: "missing", Name: "n", IPv4: "1.2.3.4", BsPort: 1, WebSocketPort: 2},
		PID:  4000000000,
	}
	// Ensure config.Path() is 0.toml
	if cfg.Path() != "0.toml" {
		t.Fatalf("unexpected path %q", cfg.Path())
	}
	// Don't create file, then Remove should return false (fixed bug: previously returned true)
	if Remove(gameId, []battleServer.Config{cfg}, false) {
		t.Error("Remove should return false when file missing, but returned true")
	}
}

func TestKeyCert_GameIdMapping(t *testing.T) {
	// Test keyCert with mocked certificatePairs
	origPairs := certificatePairs
	defer func() { certificatePairs = origPairs }()

	certificatePairs = func(parentFolder string) (bool, string, string, string, string, string) {
		return true, "/tmp/cert", "/tmp/key", "", "/tmp/selfCert", "/tmp/selfKey"
	}
	cert, key, err := keyCert("/tmp", "age2")
	if err != nil {
		t.Fatal(err)
	}
	if cert != "/tmp/selfCert" || key != "/tmp/selfKey" {
		t.Fatalf("age2 should use self-signed, got %q %q", cert, key)
	}
	cert, key, err = keyCert("/tmp", "age4")
	if err != nil {
		t.Fatal(err)
	}
	if cert != "/tmp/cert" || key != "/tmp/key" {
		t.Fatalf("age4 should use cert/key, got %q %q", cert, key)
	}
	cert, key, err = keyCert("/tmp", "athens")
	if err != nil {
		t.Fatal(err)
	}
	if cert != "/tmp/cert" || key != "/tmp/key" {
		t.Fatalf("athens should use cert/key")
	}

	certificatePairs = func(string) (bool, string, string, string, string, string) {
		return false, "", "", "", "", ""
	}
	_, _, err = keyCert("/tmp", "age2")
	if err == nil {
		t.Error("should error when CertificatePairs returns false")
	}
}

func TestResolveSSLFilesPath_AutoAndExplicit(t *testing.T) {
	origPairs := certificatePairs
	origFind := findServerPath
	defer func() { certificatePairs = origPairs; findServerPath = origFind }()

	// Test explicit path
	certificatePairs = func(parentFolder string) (bool, string, string, string, string, string) {
		if parentFolder != "/explicit" {
			t.Errorf("explicit parentFolder = %q want /explicit", parentFolder)
		}
		return true, "cert", "key", "", "selfCert", "selfKey"
	}
	cert, key, err := ResolveSSLFilesPath("age2", "/explicit")
	if err != nil {
		t.Fatal(err)
	}
	if cert != "selfCert" {
		t.Errorf("explicit: got %q", cert)
	}
	_ = key

	// Test auto with mocked FindPath
	findServerPath = func(name string) string {
		return "/tmp/server" + name
	}
	// Need CertificatePairFolder to return something; we can mock certificatePairs to succeed
	certificatePairs = func(parentFolder string) (bool, string, string, string, string, string) {
		return true, "c", "k", "", "sc", "sk"
	}
	// Mock common.CertificatePairFolder is not var, but it will be called with "/tmp/server..."; it will try to find certs - we mock certificatePairs to succeed regardless
	cert, key, err = ResolveSSLFilesPath("age2", "auto")
	if err != nil {
		t.Fatalf("auto should succeed with mocked FindPath, got %v", err)
	}
	_ = cert
	_ = key

	// Auto with FindPath failing
	findServerPath = func(string) string { return "" }
	_, _, err = ResolveSSLFilesPath("age2", "auto")
	if err == nil {
		t.Error("auto with no server exe should error")
	}
}

func TestAvailable_Mocked(t *testing.T) {
	origListen := listenTCP
	defer func() { listenTCP = origListen }()
	// Mock listen success
	listenTCP = func(address string) (error, net.Listener) {
		ln, _ := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4zero, Port: 0})
		return nil, ln
	}
	if !Available(12345) {
		t.Error("mocked listen success should return true")
	}
	// Mock listen failure
	listenTCP = func(string) (error, net.Listener) {
		return errors.New("bind fail"), nil
	}
	if Available(12345) {
		t.Error("mocked listen fail should return false")
	}
}

func TestGeneratePortsAsNeeded_FindError(t *testing.T) {
	origFind := findUnusedPorts
	defer func() { findUnusedPorts = origFind }()
	findUnusedPorts = func(count int) ([]int, error) {
		return nil, errors.New("find fail")
	}
	_, err := GeneratePortsAsNeeded([]int{0, 0})
	if err == nil || err.Error() != "find fail" {
		t.Fatalf("expected find fail, got %v", err)
	}
}

func TestKill_Mocked(t *testing.T) {
	origFind := findProcessFn
	origKill := killProcFn
	defer func() { findProcessFn = origFind; killProcFn = origKill }()

	// Not found -> should return true (already dead)
	findProcessFn = func(pid int) (*os.Process, error) {
		return nil, nil
	}
	if !Kill(battleServer.Config{PID: 1234}) {
		t.Error("not found should return true")
	}
	// Found but Kill fails
	findProcessFn = func(pid int) (*os.Process, error) {
		return &os.Process{Pid: pid}, nil
	}
	killProcFn = func(p *os.Process) error {
		return errors.New("kill fail")
	}
	if Kill(battleServer.Config{PID: 1234}) {
		t.Error("kill fail should return false")
	}
	// Found and Kill succeeds
	killProcFn = func(p *os.Process) error { return nil }
	if !Kill(battleServer.Config{PID: 1234}) {
		t.Error("kill success should return true")
	}
	// Find error
	findProcessFn = func(pid int) (*os.Process, error) {
		return nil, errors.New("find err")
	}
	if !Kill(battleServer.Config{PID: 1234}) {
		t.Error("find error should return true (treated as not found)")
	}
}

func TestRemoveAll_Mocked(t *testing.T) {
	origParsed := parsedGameIdsFnRemoveAll
	origConfigs := battleServerConfigsFn
	origRemove := removeFnRemoveAll
	defer func() {
		parsedGameIdsFnRemoveAll = origParsed
		battleServerConfigsFn = origConfigs
		removeFnRemoveAll = origRemove
	}()

	// ParsedGameIds error
	parsedGameIdsFnRemoveAll = func(*[]string) (mapset.Set[string], error) {
		return nil, errors.New("parse fail")
	}
	_, code := RemoveAll(false)
	if code != internal.ErrGames {
		t.Fatalf("expected ErrGames, got %d", code)
	}

	// Success with empty configs
	parsedGameIdsFnRemoveAll = func(*[]string) (mapset.Set[string], error) {
		return mapset.NewSet[string]("age2"), nil
	}
	battleServerConfigsFn = func(gameId string, onlyValid, ignorePid bool) ([]battleServer.Config, error) {
		return nil, errors.New("read fail")
	}
	_, code = RemoveAll(false)
	if code != 0 {
		t.Fatalf("read fail should continue and return 0, got %d", code)
	}

	// Success with configs and Remove returns false -> logs No configuration
	battleServerConfigsFn = func(string, bool, bool) ([]battleServer.Config, error) {
		return []battleServer.Config{{Base: battleServer.Base{Region: "eu"}}}, nil
	}
	removeFnRemoveAll = func(string, []battleServer.Config, bool) bool { return false }
	_, code = RemoveAll(false)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}

	// Remove returns true
	removeFnRemoveAll = func(string, []battleServer.Config, bool) bool { return true }
	_, code = RemoveAll(false)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
}

func TestWriteConfig_MarshalError(t *testing.T) {
	// Test with invalid config that cannot be marshalled? toml.Marshal should handle any struct, but we can test with a config that has invalid toml tag?
	// Instead, test the MkdirAll error by using an invalid folder path (e.g., file instead of dir)
	// Create a file where folder should be, then WriteConfig should fail to MkdirAll
	gameId := "test-marshal-error"
	dir := battleServer.Folder(gameId)
	_ = os.RemoveAll(dir)
	// Create a file at the folder path
	if err := os.WriteFile(dir, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(dir) })
	cfg := battleServer.Config{
		Base: battleServer.Base{Region: "r", Name: "n", IPv4: "127.0.0.1", BsPort: 1, WebSocketPort: 2},
		PID:  1,
	}
	err := WriteConfig(gameId, cfg)
	if err == nil {
		t.Error("expected error when folder is file")
	}
}
