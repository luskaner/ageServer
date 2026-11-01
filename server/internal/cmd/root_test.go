package cmd

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/common"
	commonCmd "github.com/luskaner/ageServer/common/cmd"
	"github.com/luskaner/ageServer/common/cmd/server"
	"github.com/luskaner/ageServer/common/fileLock"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/server/internal"
	"github.com/spf13/pflag"
)

func resetState(t *testing.T) {
	t.Helper()
	oldNewPid := fileLockNewFn
	oldLock := fileLockLockFn
	oldUnlock := fileLockUnlockFn
	oldLoggerInit := commonLoggerInitFn
	oldLoggerClose := commonLoggerCloseFn
	oldOpenLog := loggerOpenMainFileLogFn
	oldIsAdmin := isAdminFn
	oldInitCfg := initConfigFn
	oldValues := values
	oldConnectivity := internal.CanUseInternet
	oldLogger := commonLogger.FileLogger
	oldDNS := dnsConnectivityFn
	oldCache := cacheNetworkInterfacesFn
	oldSigningKeyPairCheck := signingKeyPairCheckFn
	oldCertPairFolder := certificatePairFolderFn
	oldSign := internal.SignConfigRuntime.Enabled
	oldInitGame := initializeGameFn
	t.Cleanup(func() {
		fileLockNewFn = oldNewPid
		fileLockLockFn = oldLock
		fileLockUnlockFn = oldUnlock
		commonLoggerInitFn = oldLoggerInit
		commonLoggerCloseFn = oldLoggerClose
		loggerOpenMainFileLogFn = oldOpenLog
		isAdminFn = oldIsAdmin
		initConfigFn = oldInitCfg
		values = oldValues
		internal.CanUseInternet = oldConnectivity
		commonLogger.FileLogger = oldLogger
		dnsConnectivityFn = oldDNS
		cacheNetworkInterfacesFn = oldCache
		signingKeyPairCheckFn = oldSigningKeyPairCheck
		certificatePairFolderFn = oldCertPairFolder
		internal.SignConfigRuntime.Enabled = oldSign
		initializeGameFn = oldInitGame
	})
	commonLogger.FileLogger = nil
	values = &server.Values{
		LogRootValues: &commonCmd.LogRootValues{},
		// The embedded battle server is off and the game is stubbed out: these
		// tests cover startup up to the step each one names, and a run that got
		// past that step has to return instead of binding :443 and waiting for
		// the stop signal.
		StartBsIfMissing: "false",
	}
	initializeGameFn = func(string, []internal.BattleServer) error {
		return errors.New("game initialisation is not part of this test")
	}
	commonLoggerInitFn = func(io.Writer) {}
	commonLoggerCloseFn = func() {}
	loggerOpenMainFileLogFn = func(string, bool) error { return nil }
	fileLockNewFn = func() *fileLock.PidLock { return &fileLock.PidLock{} }
	fileLockLockFn = func(*fileLock.PidLock) error { return nil }
	fileLockUnlockFn = func(*fileLock.PidLock) error { return nil }
	isAdminFn = func() bool { return true }
	// Pass by default so each test keeps exercising its own failure; the signing
	// key has its own test below.
	signingKeyPairCheckFn = func(string) error { return nil }
	initConfigFn = func(*pflag.FlagSet) (*internal.Configuration, string) {
		return &internal.Configuration{
			Log:            false,
			Internet:       internal.Internet{Enabled: true},
			Authentication: "disabled",
			Games: internal.Games{
				Enabled: []string{"age1"},
				Age1:    internal.Game{Hosts: []string{"127.0.0.1"}},
			},
			Announcement: internal.Announcement{Enabled: false},
		}, ""
	}
}

// signingConfig is the default one with Sign turned on, for the tests that care
// about what signing needs from the install.
func signingConfig(*pflag.FlagSet) (*internal.Configuration, string) {
	return &internal.Configuration{
		Log: false, Sign: internal.SignConfig{Enabled: true}, Internet: internal.Internet{Enabled: true},
		Authentication: "disabled",
		Games: internal.Games{
			Enabled: []string{"age1"},
			Age1:    internal.Game{Hosts: []string{"127.0.0.1"}},
		},
		Announcement: internal.Announcement{Enabled: false},
	}, ""
}

func TestRunRootPidLockFailure(t *testing.T) {
	resetState(t)
	fileLockLockFn = func(*fileLock.PidLock) error { return errors.New("lock fail") }
	_, code := runRoot(nil)
	if code != common.ErrPidLock {
		t.Fatalf("code=%d want ErrPidLock", code)
	}
}

func TestRunRootInvalidAuth(t *testing.T) {
	resetState(t)
	initConfigFn = func(*pflag.FlagSet) (*internal.Configuration, string) {
		return &internal.Configuration{
			Log: false, Internet: internal.Internet{Enabled: true}, Authentication: "invalid",
			Games:        internal.Games{Enabled: []string{"age1"}, Age1: internal.Game{Hosts: []string{"127.0.0.1"}}},
			Announcement: internal.Announcement{Enabled: false},
		}, ""
	}
	_, code := runRoot(nil)
	if code != internal.ErrInvalidAuthentication {
		t.Fatalf("code=%d", code)
	}
}

func TestRunRootOpenLogFailure(t *testing.T) {
	resetState(t)
	loggerOpenMainFileLogFn = func(string, bool) error { return errors.New("log fail") }
	initConfigFn = func(*pflag.FlagSet) (*internal.Configuration, string) {
		return &internal.Configuration{
			Log: true, Internet: internal.Internet{Enabled: true}, Authentication: "disabled",
			Games:        internal.Games{Enabled: []string{"age1"}, Age1: internal.Game{Hosts: []string{"127.0.0.1"}}},
			Announcement: internal.Announcement{Enabled: false},
		}, ""
	}
	values.LogRoot = "/tmp"
	_, code := runRoot(nil)
	if code != common.ErrFileLog {
		t.Fatalf("code=%d", code)
	}
}

func TestRunRootNoGames(t *testing.T) {
	resetState(t)
	initConfigFn = func(*pflag.FlagSet) (*internal.Configuration, string) {
		return &internal.Configuration{
			Log: false, Internet: internal.Internet{Enabled: true}, Authentication: "disabled",
			Games:        internal.Games{Enabled: []string{}},
			Announcement: internal.Announcement{Enabled: false},
		}, ""
	}
	values.Id = "00000000-0000-0000-0000-000000000000"
	_, code := runRoot(nil)
	if code != internal.ErrGames {
		t.Fatalf("code=%d", code)
	}
}

func TestRunRootInvalidGame(t *testing.T) {
	resetState(t)
	initConfigFn = func(*pflag.FlagSet) (*internal.Configuration, string) {
		return &internal.Configuration{
			Log: false, Internet: internal.Internet{Enabled: true}, Authentication: "disabled",
			Games:        internal.Games{Enabled: []string{"invalidGame"}},
			Announcement: internal.Announcement{Enabled: false},
		}, ""
	}
	values.Id = "00000000-0000-0000-0000-000000000000"
	_, code := runRoot(nil)
	if code != internal.ErrGames {
		t.Fatalf("code=%d", code)
	}
}

func TestRunRootInvalidID(t *testing.T) {
	resetState(t)
	initConfigFn = func(*pflag.FlagSet) (*internal.Configuration, string) {
		return &internal.Configuration{
			Log: false, Internet: internal.Internet{Enabled: true}, Authentication: "disabled",
			Games:        internal.Games{Enabled: []string{"age1"}, Age1: internal.Game{Hosts: []string{"127.0.0.1"}}},
			Announcement: internal.Announcement{Enabled: false},
		}, ""
	}
	values.Id = "bad-uuid"
	_, code := runRoot(nil)
	if code != internal.ErrInvalidId {
		t.Fatalf("code=%d want ErrInvalidId", code)
	}
}

func TestRunRootCertFolderEmpty(t *testing.T) {
	resetState(t)
	certificatePairFolderFn = func(string) string { return "" }
	values.Id = "00000000-0000-0000-0000-000000000000"
	_, code := runRoot(nil)
	if code != internal.ErrCertDirectory {
		t.Fatalf("code=%d", code)
	}
}

func TestRunRootMulticastInvalid(t *testing.T) {
	resetState(t)
	certificatePairFolderFn = func(string) string { return t.TempDir() }
	initConfigFn = func(*pflag.FlagSet) (*internal.Configuration, string) {
		return &internal.Configuration{
			Log: false, Internet: internal.Internet{Enabled: true}, Authentication: "disabled",
			Games:        internal.Games{Enabled: []string{"age1"}, Age1: internal.Game{Hosts: []string{"127.0.0.1"}}},
			Announcement: internal.Announcement{Enabled: true, Multicast: true, MulticastGroup: "999.999.999.999", Port: 8080},
		}, ""
	}
	values.Id = "00000000-0000-0000-0000-000000000000"
	_, code := runRoot(nil)
	if code != internal.ErrMulticastGroup {
		t.Fatalf("code=%d want ErrMulticastGroup %d", code, internal.ErrMulticastGroup)
	}
}

func TestRunRootRequiredAuthNoInternet(t *testing.T) {
	resetState(t)
	initConfigFn = func(*pflag.FlagSet) (*internal.Configuration, string) {
		return &internal.Configuration{
			Log: false, Internet: internal.Internet{Enabled: false}, Authentication: "required",
			Games:        internal.Games{Enabled: []string{"age1"}, Age1: internal.Game{Hosts: []string{"127.0.0.1"}}},
			Announcement: internal.Announcement{Enabled: false},
		}, ""
	}
	_, code := runRoot(nil)
	if code != internal.ErrInvalidAuthentication {
		t.Fatalf("code=%d want ErrInvalidAuthentication", code)
	}
}

func TestRunRootRequiredAuthProbeFails(t *testing.T) {
	resetState(t)
	dnsConnectivityFn = func() bool { return false }
	initConfigFn = func(*pflag.FlagSet) (*internal.Configuration, string) {
		return &internal.Configuration{
			Log: false, Internet: internal.Internet{Enabled: true}, Authentication: "required",
			Games:        internal.Games{Enabled: []string{"age1"}, Age1: internal.Game{Hosts: []string{"127.0.0.1"}}},
			Announcement: internal.Announcement{Enabled: false},
		}, ""
	}
	_, code := runRoot(nil)
	if code != internal.ErrInvalidAuthentication {
		t.Fatalf("code=%d want ErrInvalidAuthentication", code)
	}
}

func TestRunRootNoInternetPropagatesFlag(t *testing.T) {
	resetState(t)
	certificatePairFolderFn = func(string) string { return t.TempDir() }
	initConfigFn = func(*pflag.FlagSet) (*internal.Configuration, string) {
		return &internal.Configuration{
			Log: false, Internet: internal.Internet{Enabled: false}, Authentication: "disabled",
			Games:        internal.Games{Enabled: []string{"age1"}, Age1: internal.Game{Hosts: []string{"127.0.0.1"}}},
			Announcement: internal.Announcement{Enabled: true, Multicast: true, MulticastGroup: "999.999.999.999", Port: 8080},
		}, ""
	}
	values.Id = "00000000-0000-0000-0000-000000000000"
	_, code := runRoot(nil)
	if code != internal.ErrMulticastGroup {
		t.Fatalf("code=%d want ErrMulticastGroup %d", code, internal.ErrMulticastGroup)
	}
	if internal.CanUseInternet {
		t.Fatal("CanUseInternet should be false when cfg disables it")
	}
}

// A server that was told to sign but cannot is as broken as one without TLS
// material, so startup has to stop rather than serve unsigned captures.
func TestRunRootSigningKeyPairMissing(t *testing.T) {
	resetState(t)
	certificatePairFolderFn = func(string) string { return t.TempDir() }
	initConfigFn = signingConfig
	signingKeyPairCheckFn = func(folder string) error {
		return fmt.Errorf("cannot sign without %s", filepath.Join(folder, "sigkey.pem"))
	}
	values.Id = "00000000-0000-0000-0000-000000000000"
	_, code := runRoot(nil)
	if code != internal.ErrSigningKey {
		t.Fatalf("code=%d, want %d", code, internal.ErrSigningKey)
	}
}

func TestRunRootSigningKeyPairPresent(t *testing.T) {
	resetState(t)
	certificatePairFolderFn = func(string) string { return t.TempDir() }
	initConfigFn = signingConfig
	called := false
	signingKeyPairCheckFn = func(folder string) error {
		called = true
		if folder == "" {
			t.Fatal("expected the resolved certificate folder to be passed through")
		}
		return nil
	}
	values.Id = "00000000-0000-0000-0000-000000000000"
	runRoot(nil)
	if !called {
		t.Fatal("startup never checked the signing key pair")
	}
}

// With Sign off nothing is signed, so demanding a key would fail installs that
// serve perfectly fine off the captured signatures.
func TestRunRootSigningKeyPairUncheckedWhenNotSigning(t *testing.T) {
	resetState(t)
	certificatePairFolderFn = func(string) string { return t.TempDir() }
	signingKeyPairCheckFn = func(string) error {
		t.Fatal("startup demanded a signing key while Sign is off")
		return nil
	}
	values.Id = "00000000-0000-0000-0000-000000000000"
	runRoot(nil)
	if internal.SignConfigRuntime.Enabled {
		t.Fatal("Sign should stay off when the config leaves it off")
	}
}
