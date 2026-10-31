package cmd

import (
	"crypto/x509"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	commonUserData "github.com/luskaner/ageServer/launcher-common/userData"
	"github.com/luskaner/ageServer/launcher-config/internal"
)

// mockCACert implements caCertifier for tests.
type mockCACert struct {
	backupErr    error
	restoreErr   error
	restoreCerts []*x509.Certificate
	appendErr    error
}

func (m *mockCACert) Backup() error                          { return m.backupErr }
func (m *mockCACert) Restore() (error, []*x509.Certificate)  { return m.restoreErr, m.restoreCerts }
func (m *mockCACert) Append(certs []*x509.Certificate) error { return m.appendErr }

type mockFileInfo struct{ isDir bool }

func (m mockFileInfo) Name() string       { return "mock" }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() os.FileMode  { return 0755 }
func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return nil }

// saveCmdDeps saves all DI vars and restores on cleanup.
func saveCmdDeps(t *testing.T) {
	t.Helper()
	oldIsAdmin := isAdminFn
	oldConnect := connectAgentFn
	oldConnectRetries := connectAgentRetriesFn
	oldRunSetUp := runSetUpAdminFn
	oldRunRevert := runRevertAdminFn
	oldRunFlush := runFlushCacheAdminFn
	oldStartAgent := startAgentFn
	oldStopAgent := stopAgentIfNeededFn
	oldAgentRunning := configAdminAgentRunningFn
	oldRemoveCerts := removeUserCertsFn
	oldAddCerts := addUserCertsFn
	oldNewCACert := newCACertFn
	oldInitialize := initializeFn
	oldMetadata := metadataFn
	oldMetadataBackup := metadataBackupFn
	oldMetadataRestore := metadataRestoreFn
	oldBackupProfiles := backupProfilesFn
	oldRestoreProfiles := restoreProfilesFn
	oldAddHosts := addHostsFn
	oldBytesToCert := bytesToCertFn
	oldWriteAsPem := writeAsPemFn
	oldCreateFile := createFileFn
	oldRemoveFile := removeFileFn
	oldStat := statFn
	oldSupported := supportedGamesContainsFn
	t.Cleanup(func() {
		isAdminFn = oldIsAdmin
		connectAgentFn = oldConnect
		connectAgentRetriesFn = oldConnectRetries
		runSetUpAdminFn = oldRunSetUp
		runRevertAdminFn = oldRunRevert
		runFlushCacheAdminFn = oldRunFlush
		startAgentFn = oldStartAgent
		stopAgentIfNeededFn = oldStopAgent
		configAdminAgentRunningFn = oldAgentRunning
		removeUserCertsFn = oldRemoveCerts
		addUserCertsFn = oldAddCerts
		newCACertFn = oldNewCACert
		initializeFn = oldInitialize
		metadataFn = oldMetadata
		metadataBackupFn = oldMetadataBackup
		metadataRestoreFn = oldMetadataRestore
		backupProfilesFn = oldBackupProfiles
		restoreProfilesFn = oldRestoreProfiles
		addHostsFn = oldAddHosts
		bytesToCertFn = oldBytesToCert
		writeAsPemFn = oldWriteAsPem
		createFileFn = oldCreateFile
		removeFileFn = oldRemoveFile
		statFn = oldStat
		supportedGamesContainsFn = oldSupported
	})
}

// successResult returns an exec.Result that passes Success().
func successResult() *exec.Result { return &exec.Result{ExitCode: common.ErrSuccess} }
func failureResult() *exec.Result {
	return &exec.Result{Err: errors.New("exec failed"), ExitCode: 1}
}
func startAgentSuccessResult() *exec.Result {
	return &exec.Result{ExitCode: common.ErrSuccess, Pid: 1234}
}

var dummyCert = &x509.Certificate{Raw: []byte("dummy")}

// ---------------------------------------------------------------------------
// flushCache tests
// ---------------------------------------------------------------------------

func TestRunFlushCacheAdminSuccess(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return true }
	runFlushCacheAdminFn = func(string, bool, bool) (error, int) { return nil, common.ErrSuccess }
	initializeFn = func(string) error { return nil }

	_, exitCode := runFlushCache([]string{"--flushIpCache"})
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want %d", exitCode, common.ErrSuccess)
	}
}

func TestRunFlushCacheAdminFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return true }
	runFlushCacheAdminFn = func(string, bool, bool) (error, int) { return errors.New("admin failed"), 1 }
	initializeFn = func(string) error { return nil }

	_, exitCode := runFlushCache([]string{"--flushIpCache"})
	if exitCode != internal.ErrAdminSetup {
		t.Fatalf("exitCode = %d, want ErrAdminSetup %d", exitCode, internal.ErrAdminSetup)
	}
}

func TestRunFlushCacheAgentAlreadyStarted(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return false }
	connectAgentFn = func() error { return nil } // already connected => agentStarted true
	initializeFn = func(string) error { return nil }

	_, exitCode := runFlushCache([]string{"--flushIpCache"})
	if exitCode != internal.ErrAgentAlreadyStarted {
		t.Fatalf("exitCode = %d, want ErrAgentAlreadyStarted %d", exitCode, internal.ErrAgentAlreadyStarted)
	}
}

func TestRunFlushCacheAgentStartFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return false }
	connectAgentFn = func() error { return errors.New("not connected") }
	startAgentFn = func(bool, bool) *exec.Result { return failureResult() }
	initializeFn = func(string) error { return nil }

	_, exitCode := runFlushCache([]string{"--flushIpCache"})
	if exitCode != internal.ErrStartAgent {
		t.Fatalf("exitCode = %d, want ErrStartAgent %d", exitCode, internal.ErrStartAgent)
	}
}

func TestRunFlushCacheAgentStartFailureNilResult(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return false }
	connectAgentFn = func() error { return errors.New("not connected") }
	startAgentFn = func(bool, bool) *exec.Result { return &exec.Result{Err: errors.New("nil"), ExitCode: 1} }
	initializeFn = func(string) error { return nil }

	_, exitCode := runFlushCache([]string{"--flushIpCache"})
	if exitCode != internal.ErrStartAgent {
		t.Fatalf("exitCode = %d, want ErrStartAgent", exitCode)
	}
}

func TestRunFlushCacheAgentConnectVerifyFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return false }
	connectAgentFn = func() error { return errors.New("not connected") }
	startAgentFn = func(bool, bool) *exec.Result { return startAgentSuccessResult() }
	connectAgentRetriesFn = func() bool { return false }
	stopAgentIfNeededFn = func() bool { return true }
	initializeFn = func(string) error { return nil }

	_, exitCode := runFlushCache([]string{"--flushIpCache"})
	if exitCode != internal.ErrStartAgentVerify {
		t.Fatalf("exitCode = %d, want ErrStartAgentVerify %d", exitCode, internal.ErrStartAgentVerify)
	}
}

func TestRunFlushCacheAgentSuccess(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return false }
	connectAgentFn = func() error { return errors.New("not connected") }
	startAgentFn = func(bool, bool) *exec.Result { return startAgentSuccessResult() }
	connectAgentRetriesFn = func() bool { return true }
	initializeFn = func(string) error { return nil }

	_, exitCode := runFlushCache([]string{"--flushIpCache", "--flushCertsCache"})
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestRunFlushCacheBothFlagsAdmin(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return true }
	called := false
	runFlushCacheAdminFn = func(logRoot string, ips, certs bool) (error, int) {
		called = true
		if !ips || !certs {
			t.Error("expected both ips and certs true")
		}
		return nil, common.ErrSuccess
	}
	initializeFn = func(string) error { return nil }
	_, exitCode := runFlushCache([]string{"--flushIpCache", "--flushCertsCache"})
	if !called {
		t.Fatal("runFlushCacheAdminFn not called")
	}
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

// ---------------------------------------------------------------------------
// stopAgent tests
// ---------------------------------------------------------------------------

func TestRunStopAgentCallsStop(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	called := false
	stopAgentIfNeededFn = func() bool { called = true; return true }
	_, exitCode := runStopAgent([]string{})
	if !called {
		t.Fatal("stopAgentIfNeededFn not called")
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

// ---------------------------------------------------------------------------
// revert tests - flag parsing and early returns
// ---------------------------------------------------------------------------

func TestRunRevertAoE1DisablesMetadataAndCA(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	// AoE1 should force Metadata=false and RestoreCAStoreCert=false.
	// We trigger the full flow but mock everything to success, then check flags.
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return []*x509.Certificate{dummyCert}, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{} }
	initializeFn = func(string) error { return nil }
	// No admin elevation needed when no IPs/Certs
	_, exitCode := runRevert([]string{"--game", "age1", "--metadata"})
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	// Metadata should have been disabled, so no restore should have been attempted via stat? Actually AoE1 sets Metadata false, so no dataPath validation.
	// If we pass --metadata with age1, it should be ignored and succeed.
}

func TestRunRevertAoE4DisablesCA(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return []*x509.Certificate{dummyCert}, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	initializeFn = func(string) error { return nil }
	// Age4 with --caStoreCert should have it disabled, so no gamePath needed.
	_, exitCode := runRevert([]string{"--game", "age4", "--caStoreCert", "--gamePath", ""})
	// Should not require gamePath because flag disabled
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0, revert should skip CA for age4", exitCode)
	}
}

func TestRunRevertInvalidGame(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return false }
	initializeFn = func(string) error { return nil }
	_, exitCode := runRevert([]string{"--game", "age2", "--metadata"})
	if exitCode != launcherCommon.ErrInvalidGame {
		t.Fatalf("exitCode = %d, want ErrInvalidGame %d", exitCode, launcherCommon.ErrInvalidGame)
	}
}

func TestRunRevertInvalidDataPath(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	initializeFn = func(string) error { return nil }
	_, exitCode := runRevert([]string{"--game", "age2", "--metadata"})
	if exitCode != internal.ErrInvalidDataPath {
		t.Fatalf("exitCode = %d, want ErrInvalidDataPath %d", exitCode, internal.ErrInvalidDataPath)
	}
}

func TestRunRevertRemoveUserCertFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return nil, errors.New("fail") }
	initializeFn = func(string) error { return nil }
	_, exitCode := runRevert([]string{"--game", "age2", "--userCert"})
	if exitCode != internal.ErrUserCertRemove {
		t.Fatalf("exitCode = %d, want ErrUserCertRemove %d", exitCode, internal.ErrUserCertRemove)
	}
}

func TestRunRevertMetadataRestoreFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return false }
	initializeFn = func(string) error { return nil }
	_, exitCode := runRevert([]string{"--game", "age2", "--metadata"})
	if exitCode != internal.ErrMetadataRestore {
		t.Fatalf("exitCode = %d, want ErrMetadataRestore %d", exitCode, internal.ErrMetadataRestore)
	}
}

func TestRunRevertProfilesRestoreFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return false }
	initializeFn = func(string) error { return nil }
	_, exitCode := runRevert([]string{"--game", "age2", "--profiles"})
	if exitCode != internal.ErrProfilesRestore {
		t.Fatalf("exitCode = %d, want ErrProfilesRestore %d", exitCode, internal.ErrProfilesRestore)
	}
}

func TestRunRevertGamePathMissing(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	_, exitCode := runRevert([]string{"--game", "age2", "--caStoreCert"})
	if exitCode != internal.ErrGamePathMissing {
		t.Fatalf("exitCode = %d, want ErrGamePathMissing %d", exitCode, internal.ErrGamePathMissing)
	}
}

func TestRunRevertGameCertRestoreFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{restoreErr: errors.New("restore fail")} }
	initializeFn = func(string) error { return nil }
	_, exitCode := runRevert([]string{"--game", "age2", "--caStoreCert", "--gamePath", "/tmp"})
	if exitCode != internal.ErrGameCertRestore {
		t.Fatalf("exitCode = %d, want ErrGameCertRestore %d", exitCode, internal.ErrGameCertRestore)
	}
}

func TestRunRevertAdminSuccess(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	isAdminFn = func() bool { return true }
	connectAgentFn = func() error { return errors.New("not connected") }
	runRevertAdminFn = func(string, bool, bool, bool) (error, int) { return nil, common.ErrSuccess }
	stopAgentIfNeededFn = func() bool { return true }
	initializeFn = func(string) error { return nil }
	removeFileFn = func(string) error { return nil }
	_, exitCode := runRevert([]string{"--game", "age2", "--ip"})
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestRunRevertAdminFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	isAdminFn = func() bool { return true }
	connectAgentFn = func() error { return errors.New("not connected") }
	runRevertAdminFn = func(string, bool, bool, bool) (error, int) { return errors.New("fail"), 1 }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true } // to avoid early
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return []*x509.Certificate{dummyCert}, nil }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{} }
	initializeFn = func(string) error { return nil }
	// Trigger admin path via --ip (requires admin)
	// But we also need to ensure RemoveAll false, so RevertRequiresAdminElevationValues returns true for IPs
	_, exitCode := runRevert([]string{"--game", "age2", "--ip"})
	if exitCode != internal.ErrAdminRevert {
		t.Fatalf("exitCode = %d, want ErrAdminRevert %d", exitCode, internal.ErrAdminRevert)
	}
}

// Regression: the 'config-admin-agent' shutdown used to be the last statement
// of the happy path, so the ErrAdminRevert early return above leaked an
// elevated process that only the user could kill.
func TestRunRevertAdminFailureStopsAgent(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	isAdminFn = func() bool { return true }
	connectAgentFn = func() error { return errors.New("not connected") }
	runRevertAdminFn = func(string, bool, bool, bool) (error, int) { return errors.New("fail"), 1 }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{} }
	initializeFn = func(string) error { return nil }
	configAdminAgentRunningFn = func() bool { return true }
	stopped := false
	stopAgentIfNeededFn = func() bool { stopped = true; return true }

	if _, exitCode := runRevert([]string{"--game", "age2", "--ip"}); exitCode != internal.ErrAdminRevert {
		t.Fatalf("exitCode = %d, want ErrAdminRevert %d", exitCode, internal.ErrAdminRevert)
	}
	if !stopped {
		t.Error("config-admin-agent was left running after a failed revert")
	}
}

// Regression: the stop was gated on agentConnected, which is only allocated
// inside the "revert needs elevation" branch. A running agent was therefore
// never stopped for configs using a custom hosts/cert file.
func TestRunRevertStopsAgentWithoutAdminElevation(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return true }
	connectAgentFn = func() error { return errors.New("not connected") }
	initializeFn = func(string) error { return nil }
	configAdminAgentRunningFn = func() bool { return true }
	stopped := false
	stopAgentIfNeededFn = func() bool { stopped = true; return true }

	// No --ip and no --certs, so RevertRequiresAdminElevationValues is false.
	if _, exitCode := runRevert([]string{"--game", "age2"}); exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if !stopped {
		t.Error("config-admin-agent was left running although the revert needed no elevation")
	}
}

// No agent running means nothing to stop: the revert must not touch the IPC.
func TestRunRevertDoesNotStopAbsentAgent(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return true }
	connectAgentFn = func() error { return errors.New("not connected") }
	runRevertAdminFn = func(string, bool, bool, bool) (error, int) { return nil, common.ErrSuccess }
	initializeFn = func(string) error { return nil }
	configAdminAgentRunningFn = func() bool { return false }
	stopAgentIfNeededFn = func() bool { t.Error("stopped an agent that was not running"); return true }

	if _, exitCode := runRevert([]string{"--game", "age2", "--ip"}); exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

// A revert that succeeded but could not stop the agent must report it, so the
// caller does not record the cleanup as done.
func TestRunRevertStopAgentFailureIsReported(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return true }
	connectAgentFn = func() error { return nil }
	runRevertAdminFn = func(string, bool, bool, bool) (error, int) { return nil, common.ErrSuccess }
	initializeFn = func(string) error { return nil }
	configAdminAgentRunningFn = func() bool { return true }
	stopAgentIfNeededFn = func() bool { return false }

	if _, exitCode := runRevert([]string{"--game", "age2", "--ip"}); exitCode != internal.ErrRevertStopAgent {
		t.Fatalf("exitCode = %d, want ErrRevertStopAgent %d", exitCode, internal.ErrRevertStopAgent)
	}
}

// A stop failure must not mask an earlier, more specific failure.
func TestRunRevertStopAgentFailureKeepsOriginalExitCode(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	isAdminFn = func() bool { return true }
	connectAgentFn = func() error { return nil }
	runRevertAdminFn = func(string, bool, bool, bool) (error, int) { return errors.New("fail"), 1 }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{} }
	initializeFn = func(string) error { return nil }
	configAdminAgentRunningFn = func() bool { return true }
	stopAgentIfNeededFn = func() bool { return false }

	if _, exitCode := runRevert([]string{"--game", "age2", "--ip"}); exitCode != internal.ErrAdminRevert {
		t.Fatalf("exitCode = %d, want the original ErrAdminRevert %d", exitCode, internal.ErrAdminRevert)
	}
}

func TestRunRevertRemoveAllSuccess(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return []*x509.Certificate{dummyCert}, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{} }
	isAdminFn = func() bool { return true }
	connectAgentFn = func() error { return errors.New("not connected") }
	runRevertAdminFn = func(string, bool, bool, bool) (error, int) { return nil, common.ErrSuccess }
	stopAgentIfNeededFn = func() bool { return true }
	removeFileFn = func(string) error { return nil }
	initializeFn = func(string) error { return nil }
	// RemoveAll triggers all flags
	_, exitCode := runRevert([]string{"--game", "age2", "--all", "--dataPath", "/tmp", "--gamePath", "/tmp"})
	// Should succeed because RemoveAll ignores failfast
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

// ---------------------------------------------------------------------------
// setUp tests
// ---------------------------------------------------------------------------

func TestRunSetUpAoE1DisablesMetadataAndCA(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	// AoE1 should force Metadata false and AddCACertData nil even if passed.
	// We pass --metadata and --caStoreCert, but they should be ignored.
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	bytesToCertFn = func([]byte) *x509.Certificate { return dummyCert }
	addUserCertsFn = func([]*x509.Certificate) error { return nil }
	metadataBackupFn = func(*commonUserData.Path) bool { t.Error("metadata backup should not be called for AoE1"); return true }
	backupProfilesFn = func(*commonUserData.Path) bool {
		t.Error("profiles backup should not be called unless requested")
		return true
	}
	newCACertFn = func(string, string) caCertifier {
		t.Error("newCACert should not be called for AoE1")
		return &mockCACert{}
	}
	// Use non-admin path with no IP/cert to avoid admin call
	_, exitCode := runSetUp([]string{"--game", "age1"})
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestRunSetUpAoE4DisablesCA(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	// AoE4 should disable AddCACertData
	newCACertFn = func(string, string) caCertifier {
		t.Error("newCACert should not be called for AoE4")
		return &mockCACert{}
	}
	_, exitCode := runSetUp([]string{"--game", "age4"})
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestRunSetUpInvalidGame(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return false }
	initializeFn = func(string) error { return nil }
	_, exitCode := runSetUp([]string{"--game", "age2", "--metadata"})
	if exitCode != launcherCommon.ErrInvalidGame {
		t.Fatalf("exitCode = %d, want ErrInvalidGame %d", exitCode, launcherCommon.ErrInvalidGame)
	}
}

func TestRunSetUpInvalidDataPath(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	initializeFn = func(string) error { return nil }
	_, exitCode := runSetUp([]string{"--game", "age2", "--metadata"})
	if exitCode != internal.ErrInvalidDataPath {
		t.Fatalf("exitCode = %d, want ErrInvalidDataPath", exitCode)
	}
}

func TestRunSetUpMissingLocalCertData(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	initializeFn = func(string) error { return nil }
	_, exitCode := runSetUp([]string{"--game", "age2", "--certFilePath", "/tmp/cert.pem"})
	if exitCode != internal.ErrMissingLocalCertData {
		t.Fatalf("exitCode = %d, want ErrMissingLocalCertData", exitCode)
	}
}

func TestRunSetUpUserCertParseFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	bytesToCertFn = func([]byte) *x509.Certificate { return nil }
	// Need to bypass data path check? No metadata/profiles false so no check.
	encoded := "dGVzdA==" // base64 "test"
	_, exitCode := runSetUp([]string{"--game", "age2", "--userCert", encoded})
	if exitCode != internal.ErrUserCertAddParse {
		t.Fatalf("exitCode = %d, want ErrUserCertAddParse %d", exitCode, internal.ErrUserCertAddParse)
	}
}

func TestRunSetUpUserCertAddFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	bytesToCertFn = func([]byte) *x509.Certificate { return dummyCert }
	addUserCertsFn = func([]*x509.Certificate) error { return errors.New("add fail") }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return nil, nil }
	removeFileFn = func(string) error { return nil }
	metadataBackupFn = func(*commonUserData.Path) bool { return true }
	backupProfilesFn = func(*commonUserData.Path) bool { return true }
	// need to handle undo
	encoded := "dGVzdA=="
	_, exitCode := runSetUp([]string{"--game", "age2", "--userCert", encoded})
	if exitCode != internal.ErrUserCertAdd {
		t.Fatalf("exitCode = %d, want ErrUserCertAdd %d", exitCode, internal.ErrUserCertAdd)
	}
}

func TestRunSetUpMetadataBackupFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	metadataBackupFn = func(*commonUserData.Path) bool { return false }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return nil, nil }
	removeFileFn = func(string) error { return nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	_, exitCode := runSetUp([]string{"--game", "age2", "--metadata", "--dataPath", "/tmp"})
	if exitCode != internal.ErrMetadataBackup {
		t.Fatalf("exitCode = %d, want ErrMetadataBackup %d", exitCode, internal.ErrMetadataBackup)
	}
}

func TestRunSetUpProfilesBackupFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	metadataBackupFn = func(*commonUserData.Path) bool { return true }
	backupProfilesFn = func(*commonUserData.Path) bool { return false }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return nil, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	removeFileFn = func(string) error { return nil }
	_, exitCode := runSetUp([]string{"--game", "age2", "--profiles", "--dataPath", "/tmp"})
	if exitCode != internal.ErrProfilesBackup {
		t.Fatalf("exitCode = %d, want ErrProfilesBackup", exitCode)
	}
}

func TestRunSetUpGamePathMissing(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	bytesToCertFn = func([]byte) *x509.Certificate { return dummyCert }
	// Need to bypass metadata/profiles, directly to CA cert
	// Provide caStoreCert via base64 "test"
	encoded := "dGVzdA=="
	_, exitCode := runSetUp([]string{"--game", "age2", "--caStoreCert", encoded})
	if exitCode != internal.ErrGamePathMissing {
		t.Fatalf("exitCode = %d, want ErrGamePathMissing %d", exitCode, internal.ErrGamePathMissing)
	}
}

func TestRunSetUpGameCertParseFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	bytesToCertFn = func([]byte) *x509.Certificate { return nil }
	removeFileFn = func(string) error { return nil }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return nil, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	encoded := "dGVzdA=="
	_, exitCode := runSetUp([]string{"--game", "age2", "--caStoreCert", encoded, "--gamePath", "/tmp"})
	if exitCode != internal.ErrGameCertAddParse {
		t.Fatalf("exitCode = %d, want ErrGameCertAddParse %d", exitCode, internal.ErrGameCertAddParse)
	}
}

func TestRunSetUpGameCertBackupFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	bytesToCertFn = func([]byte) *x509.Certificate { return dummyCert }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{backupErr: errors.New("backup fail")} }
	removeFileFn = func(string) error { return nil }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return nil, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	encoded := "dGVzdA=="
	_, exitCode := runSetUp([]string{"--game", "age2", "--caStoreCert", encoded, "--gamePath", "/tmp"})
	if exitCode != internal.ErrGameCertBackup {
		t.Fatalf("exitCode = %d, want ErrGameCertBackup %d", exitCode, internal.ErrGameCertBackup)
	}
}

func TestRunSetUpGameCertAppendFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	bytesToCertFn = func([]byte) *x509.Certificate { return dummyCert }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{appendErr: errors.New("append fail")} }
	removeFileFn = func(string) error { return nil }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return nil, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	encoded := "dGVzdA=="
	_, exitCode := runSetUp([]string{"--game", "age2", "--caStoreCert", encoded, "--gamePath", "/tmp"})
	if exitCode != internal.ErrGameCertAdd {
		t.Fatalf("exitCode = %d, want ErrGameCertAdd %d", exitCode, internal.ErrGameCertAdd)
	}
}

func TestRunSetUpHostsAddFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	addHostsFn = func(net.IP, string, string, string, bool, func() *exec.Result) (bool, error) {
		return false, errors.New("hosts fail")
	}
	removeFileFn = func(string) error { return nil }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return nil, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{} }
	_, exitCode := runSetUp([]string{"--game", "age2", "--ip", "127.0.0.2", "--hostFilePath", "/tmp/hosts"})
	if exitCode != internal.ErrHostsAdd {
		t.Fatalf("exitCode = %d, want ErrHostsAdd %d", exitCode, internal.ErrHostsAdd)
	}
}

func TestRunSetUpCertFileCreateFailure(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	createFileFn = func(string) (*os.File, error) { return nil, errors.New("create fail") }
	removeFileFn = func(string) error { return nil }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return nil, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{} }
	// Need localCert data for file creation path trigger
	encodedLocal := "dGVzdA==" // "test"
	// Also need to provide host file? Actually certFilePath triggers write
	_, exitCode := runSetUp([]string{"--game", "age2", "--certFilePath", "/tmp/cert.pem", "--localCert", encodedLocal})
	if exitCode != internal.ErrUserCertAdd {
		t.Fatalf("exitCode = %d, want ErrUserCertAdd", exitCode)
	}
}

func TestRunSetUpAdminSuccessViaAgent(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	connectAgentFn = func() error { return nil } // agent connected
	gotCanUseInternet := true
	runSetUpAdminFn = func(_ string, _ string, _ net.IP, _ bool, _ []byte, canUseInternet bool) (error, int) {
		gotCanUseInternet = canUseInternet
		return nil, common.ErrSuccess
	}
	stopAgentIfNeededFn = func() bool { return true }
	removeFileFn = func(string) error { return nil }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return nil, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{} }
	_, exitCode := runSetUp([]string{"--game", "age2", "--ip", "127.0.0.2", "--internet=false"})
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if gotCanUseInternet {
		t.Error("canUseInternet should be forwarded as false to runSetUpAdminFn")
	}
}

func TestRunSetUpAdminFailureViaAgent(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	statFn = func(string) (os.FileInfo, error) { return mockFileInfo{isDir: true}, nil }
	initializeFn = func(string) error { return nil }
	connectAgentFn = func() error { return nil } // agent connected
	runSetUpAdminFn = func(string, string, net.IP, bool, []byte, bool) (error, int) { return errors.New("admin fail"), 1 }
	removeFileFn = func(string) error { return nil }
	removeUserCertsFn = func() ([]*x509.Certificate, error) { return nil, nil }
	metadataRestoreFn = func(*commonUserData.Path) bool { return true }
	restoreProfilesFn = func(*commonUserData.Path, bool) bool { return true }
	newCACertFn = func(string, string) caCertifier { return &mockCACert{} }
	// AgentEndOnError defaults false, so undo should be called but not stopAgent
	_, exitCode := runSetUp([]string{"--game", "age2", "--ip", "127.0.0.2"})
	if exitCode != internal.ErrAdminSetup {
		t.Fatalf("exitCode = %d, want ErrAdminSetup %d", exitCode, internal.ErrAdminSetup)
	}
}

func TestRunSetUpSuccessMinimal(t *testing.T) {
	resetCmdState(t)
	saveCmdDeps(t)
	supportedGamesContainsFn = func(string) bool { return true }
	initializeFn = func(string) error { return nil }
	// Minimal setup with no optional features should succeed
	_, exitCode := runSetUp([]string{"--game", "age2"})
	if exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}
