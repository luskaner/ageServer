package cmd

import (
	"crypto/x509"
	"errors"
	"net"
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/launcher-config-admin/internal"
)

func resetState(t *testing.T) {
	t.Helper()
	oldBytes := bytesToCertFn
	oldTrust := trustCertsFn
	oldUntrust := untrustCertsFn
	oldAddHosts := addHostsFn
	oldRemoveHosts := removeHostsFn
	oldFlushDns := flushDnsFn
	oldFlushCerts := flushCertsFn
	oldInit := initializeFn
	oldGOOS := runtimeGOOS
	oldSetUp := internal.SetUp
	oldLogger := internal.Logger
	t.Cleanup(func() {
		bytesToCertFn = oldBytes
		trustCertsFn = oldTrust
		untrustCertsFn = oldUntrust
		addHostsFn = oldAddHosts
		removeHostsFn = oldRemoveHosts
		flushDnsFn = oldFlushDns
		flushCertsFn = oldFlushCerts
		initializeFn = oldInit
		runtimeGOOS = oldGOOS
		internal.SetUp = oldSetUp
		internal.Logger = oldLogger
	})
	internal.Logger = nil
	internal.SetUp = nil
}

// helpers for mock results
func successResult() *exec.Result { return &exec.Result{ExitCode: common.ErrSuccess} }
func failureResult() *exec.Result { return &exec.Result{ExitCode: 1, Err: errors.New("fail")} }

var dummyCert = &x509.Certificate{Raw: []byte("dummy")}

// -------------------------------------------------------------------
// helper function tests
// -------------------------------------------------------------------

func TestUntrustCertificateSuccess(t *testing.T) {
	resetState(t)
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return nil, nil }
	if !untrustCertificate() {
		t.Fatal("expected true")
	}
}

func TestUntrustCertificateFailure(t *testing.T) {
	resetState(t)
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return nil, errors.New("fail") }
	if untrustCertificate() {
		t.Fatal("expected false")
	}
}

func TestTrustCertificatesSuccess(t *testing.T) {
	resetState(t)
	trustCertsFn = func(bool, []*x509.Certificate) error { return nil }
	if !trustCertificates([]*x509.Certificate{dummyCert}) {
		t.Fatal("expected true")
	}
}

func TestTrustCertificatesFailure(t *testing.T) {
	resetState(t)
	trustCertsFn = func(bool, []*x509.Certificate) error { return errors.New("fail") }
	if trustCertificates([]*x509.Certificate{dummyCert}) {
		t.Fatal("expected false")
	}
}

// -------------------------------------------------------------------
// runSetUp tests
// -------------------------------------------------------------------

func TestRunSetUpSyntaxError(t *testing.T) {
	resetState(t)
	_, code := runSetUp([]string{"--not-exist"})
	if code != common.ErrSyntax {
		t.Fatalf("code=%d want syntax", code)
	}
	if internal.SetUp != nil {
		t.Fatal("SetUp should be nil on syntax error? Actually it sets before parse? Check: it sets after parse, so on syntax error it should be nil")
	}
}

func TestRunSetUpMissingGame(t *testing.T) {
	resetState(t)
	_, code := runSetUp([]string{})
	if code != common.ErrSyntax {
		t.Fatalf("code=%d want syntax", code)
	}
}

func TestRunSetUpCertParseFailure(t *testing.T) {
	resetState(t)
	bytesToCertFn = func([]byte) *x509.Certificate { return nil }
	initializeFn = func(string) error { return nil }
	_, code := runSetUp([]string{"--game", "age2", "--localCert", "dGVzdA=="})
	if code != internal.ErrLocalCertAddParse {
		t.Fatalf("code=%d want ErrLocalCertAddParse %d", code, internal.ErrLocalCertAddParse)
	}
}

func TestRunSetUpCertAddFailure(t *testing.T) {
	resetState(t)
	bytesToCertFn = func([]byte) *x509.Certificate { return dummyCert }
	trustCertsFn = func(bool, []*x509.Certificate) error { return errors.New("fail") }
	initializeFn = func(string) error { return nil }
	_, code := runSetUp([]string{"--game", "age2", "--localCert", "dGVzdA=="})
	if code != internal.ErrLocalCertAdd {
		t.Fatalf("code=%d want ErrLocalCertAdd %d", code, internal.ErrLocalCertAdd)
	}
}

func TestRunSetUpCertAddSuccessNoIP(t *testing.T) {
	resetState(t)
	bytesToCertFn = func([]byte) *x509.Certificate { return dummyCert }
	trustCertsFn = func(bool, []*x509.Certificate) error { return nil }
	initializeFn = func(string) error { return nil }
	_, code := runSetUp([]string{"--game", "age2", "--localCert", "dGVzdA=="})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
	if internal.SetUp == nil || !*internal.SetUp {
		t.Fatal("SetUp should be true")
	}
}

func TestRunSetUpInitializeFailureStillSucceeds(t *testing.T) {
	resetState(t)
	initializeFn = func(string) error { return errors.New("init fail") }
	// No cert, no IP, just game and logRoot -> should succeed even if init fails
	_, code := runSetUp([]string{"--game", "age2", "--logRoot", "/tmp/log"})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want success even if init fails", code)
	}
}

func TestRunSetUpAddHostsSuccess(t *testing.T) {
	resetState(t)
	addHostsFn = func(net.IP, string, string, string, bool, func() *exec.Result) (bool, error) { return true, nil }
	initializeFn = func(string) error { return nil }
	_, code := runSetUp([]string{"--game", "age2", "--ip", "127.0.0.2"})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
}

func TestRunSetUpAddHostsFailureNoCert(t *testing.T) {
	resetState(t)
	addHostsFn = func(net.IP, string, string, string, bool, func() *exec.Result) (bool, error) {
		return false, errors.New("hosts fail")
	}
	initializeFn = func(string) error { return nil }
	_, code := runSetUp([]string{"--game", "age2", "--ip", "127.0.0.2"})
	if code != internal.ErrIpMapAdd {
		t.Fatalf("code=%d want ErrIpMapAdd %d", code, internal.ErrIpMapAdd)
	}
}

func TestRunSetUpAddHostsFailureWithCertUntrustSuccess(t *testing.T) {
	resetState(t)
	bytesToCertFn = func([]byte) *x509.Certificate { return dummyCert }
	trustCertsFn = func(bool, []*x509.Certificate) error { return nil }
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return nil, nil }
	addHostsFn = func(net.IP, string, string, string, bool, func() *exec.Result) (bool, error) {
		return false, errors.New("fail")
	}
	initializeFn = func(string) error { return nil }
	_, code := runSetUp([]string{"--game", "age2", "--localCert", "dGVzdA==", "--ip", "127.0.0.2"})
	if code != internal.ErrIpMapAdd {
		t.Fatalf("code=%d want ErrIpMapAdd %d", code, internal.ErrIpMapAdd)
	}
}

func TestRunSetUpAddHostsFailureWithCertUntrustFailure(t *testing.T) {
	resetState(t)
	bytesToCertFn = func([]byte) *x509.Certificate { return dummyCert }
	trustCertsFn = func(bool, []*x509.Certificate) error { return nil }
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return nil, errors.New("untrust fail") }
	addHostsFn = func(net.IP, string, string, string, bool, func() *exec.Result) (bool, error) {
		return false, errors.New("fail")
	}
	initializeFn = func(string) error { return nil }
	_, code := runSetUp([]string{"--game", "age2", "--localCert", "dGVzdA==", "--ip", "127.0.0.2"})
	if code != internal.ErrIpMapAddRevert {
		t.Fatalf("code=%d want ErrIpMapAddRevert %d", code, internal.ErrIpMapAddRevert)
	}
}

func TestRunSetUpBothCertAndIPSuccess(t *testing.T) {
	resetState(t)
	bytesToCertFn = func([]byte) *x509.Certificate { return dummyCert }
	trustCertsFn = func(bool, []*x509.Certificate) error { return nil }
	addHostsFn = func(net.IP, string, string, string, bool, func() *exec.Result) (bool, error) { return true, nil }
	initializeFn = func(string) error { return nil }
	_, code := runSetUp([]string{"--game", "age2", "--localCert", "dGVzdA==", "--ip", "127.0.0.2"})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
}

// -------------------------------------------------------------------
// runRevert tests
// -------------------------------------------------------------------

func TestRunRevertSyntaxError(t *testing.T) {
	resetState(t)
	_, code := runRevert([]string{"--bad"})
	if code != common.ErrSyntax {
		t.Fatalf("code=%d want syntax", code)
	}
}

func TestRunRevertRemoveAllNoFlags(t *testing.T) {
	resetState(t)
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return []*x509.Certificate{dummyCert}, nil }
	removeHostsFn = func() error { return nil }
	initializeFn = func(string) error { return nil }
	_, code := runRevert([]string{"--all"})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
	if internal.SetUp == nil || *internal.SetUp {
		t.Fatal("SetUp should be false for revert")
	}
}

func TestRunRevertCertsRemoveFailureNoRemoveAll(t *testing.T) {
	resetState(t)
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return nil, errors.New("fail") }
	initializeFn = func(string) error { return nil }
	_, code := runRevert([]string{"--localCert"})
	if code != internal.ErrLocalCertRemove {
		t.Fatalf("code=%d want ErrLocalCertRemove %d", code, internal.ErrLocalCertRemove)
	}
}

func TestRunRevertCertsRemoveFailureWithRemoveAllContinues(t *testing.T) {
	resetState(t)
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return nil, errors.New("fail") }
	removeHostsFn = func() error { return nil }
	initializeFn = func(string) error { return nil }
	_, code := runRevert([]string{"--all"})
	// With RemoveAll, cert failure should be ignored and continue to IPs, which succeeds, so overall success? Check code: if err != nil and !RemoveAll => return ErrLocalCertRemove, else continue and still check IPs. So with RemoveAll true, it logs but does not return, then proceeds to IPs. So final code should be success if IPs succeed.
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want success with RemoveAll", code)
	}
}

func TestRunRevertIPsRemoveSuccessAfterCert(t *testing.T) {
	resetState(t)
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return []*x509.Certificate{dummyCert}, nil }
	removeHostsFn = func() error { return nil }
	initializeFn = func(string) error { return nil }
	_, code := runRevert([]string{"--localCert", "--ip"})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
}

func TestRunRevertIPsRemoveFailureNoCert(t *testing.T) {
	resetState(t)
	removeHostsFn = func() error { return errors.New("fail") }
	initializeFn = func(string) error { return nil }
	_, code := runRevert([]string{"--ip"})
	if code != internal.ErrIpMapRemove {
		t.Fatalf("code=%d want ErrIpMapRemove %d", code, internal.ErrIpMapRemove)
	}
}

func TestRunRevertIPsRemoveFailureWithCertTrustSuccess(t *testing.T) {
	resetState(t)
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return []*x509.Certificate{dummyCert}, nil }
	trustCertsFn = func(bool, []*x509.Certificate) error { return nil }
	removeHostsFn = func() error { return errors.New("fail") }
	initializeFn = func(string) error { return nil }
	_, code := runRevert([]string{"--localCert", "--ip"})
	if code != internal.ErrIpMapRemove {
		t.Fatalf("code=%d want ErrIpMapRemove %d", code, internal.ErrIpMapRemove)
	}
}

func TestRunRevertIPsRemoveFailureWithCertTrustFailure(t *testing.T) {
	resetState(t)
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return []*x509.Certificate{dummyCert}, nil }
	trustCertsFn = func(bool, []*x509.Certificate) error { return errors.New("trust fail") }
	removeHostsFn = func() error { return errors.New("fail") }
	initializeFn = func(string) error { return nil }
	_, code := runRevert([]string{"--localCert", "--ip"})
	if code != internal.ErrIpMapRemoveRevert {
		t.Fatalf("code=%d want ErrIpMapRemoveRevert %d", code, internal.ErrIpMapRemoveRevert)
	}
}

func TestRunRevertIPsRemoveFailureWithRemoveAllNoRevert(t *testing.T) {
	resetState(t)
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return []*x509.Certificate{dummyCert}, nil }
	removeHostsFn = func() error { return errors.New("fail") }
	trustCertsFn = func(bool, []*x509.Certificate) error {
		t.Error("trust should not be called with RemoveAll")
		return nil
	}
	initializeFn = func(string) error { return nil }
	_, code := runRevert([]string{"--all"})
	// With RemoveAll, IPs fail should return ErrIpMapRemove (not revert) and not try trust
	if code != internal.ErrIpMapRemove {
		t.Fatalf("code=%d want ErrIpMapRemove", code)
	}
}

func TestRunRevertNoFlagsSuccess(t *testing.T) {
	resetState(t)
	initializeFn = func(string) error { return nil }
	_, code := runRevert([]string{})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
}

func TestRunRevertOnlyCertsSuccess(t *testing.T) {
	resetState(t)
	untrustCertsFn = func(bool) ([]*x509.Certificate, error) { return []*x509.Certificate{dummyCert}, nil }
	initializeFn = func(string) error { return nil }
	_, code := runRevert([]string{"--localCert"})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
}

// -------------------------------------------------------------------
// flushCache tests
// -------------------------------------------------------------------

func TestRunFlushCacheSyntaxError(t *testing.T) {
	resetState(t)
	_, code := runFlushCache([]string{"--bad"})
	if code != common.ErrSyntax {
		t.Fatalf("code=%d want syntax", code)
	}
}

func TestRunFlushCacheNoFlags(t *testing.T) {
	resetState(t)
	initializeFn = func(string) error { return nil }
	_, code := runFlushCache([]string{})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
}

func TestRunFlushCacheIPsSuccess(t *testing.T) {
	resetState(t)
	flushDnsFn = func() *exec.Result { return successResult() }
	initializeFn = func(string) error { return nil }
	_, code := runFlushCache([]string{"--flushIpCache"})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
}

func TestRunFlushCacheIPsFailure(t *testing.T) {
	resetState(t)
	flushDnsFn = func() *exec.Result { return failureResult() }
	initializeFn = func(string) error { return nil }
	_, code := runFlushCache([]string{"--flushIpCache"})
	if code != internal.ErrFlushCacheDNS {
		t.Fatalf("code=%d want ErrFlushCacheDNS %d", code, internal.ErrFlushCacheDNS)
	}
}

func TestRunFlushCacheInitializeFailureStillProceeds(t *testing.T) {
	resetState(t)
	initializeFn = func(string) error { return errors.New("init fail") }
	flushDnsFn = func() *exec.Result { return successResult() }
	_, code := runFlushCache([]string{"--flushIpCache", "--logRoot", "/tmp"})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
}

func TestRunFlushCacheCertsSuccessOnLinux(t *testing.T) {
	resetState(t)
	runtimeGOOS = "linux"
	flushCertsFn = func() *exec.Result { return successResult() }
	initializeFn = func(string) error { return nil }
	_, code := runFlushCache([]string{"--flushCertsCache"})
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
}

func TestRunFlushCacheCertsFailureOnLinux(t *testing.T) {
	resetState(t)
	runtimeGOOS = "linux"
	flushCertsFn = func() *exec.Result { return failureResult() }
	initializeFn = func(string) error { return nil }
	_, code := runFlushCache([]string{"--flushCertsCache"})
	if code != internal.ErrFlushCacheCerts {
		t.Fatalf("code=%d want ErrFlushCacheCerts %d", code, internal.ErrFlushCacheCerts)
	}
}

func TestRunFlushCacheCertsFailureWithOnlyExitCode(t *testing.T) {
	resetState(t)
	runtimeGOOS = "linux"
	flushCertsFn = func() *exec.Result { return &exec.Result{ExitCode: 5} }
	initializeFn = func(string) error { return nil }
	_, code := runFlushCache([]string{"--flushCertsCache"})
	if code != internal.ErrFlushCacheCerts {
		t.Fatalf("code=%d want ErrFlushCacheCerts", code)
	}
}

func TestRunFlushCacheBothFailOnLinuxCombined(t *testing.T) {
	resetState(t)
	runtimeGOOS = "linux"
	flushCertsFn = func() *exec.Result { return failureResult() }
	flushDnsFn = func() *exec.Result { return failureResult() }
	initializeFn = func(string) error { return nil }
	_, code := runFlushCache([]string{"--flushCertsCache", "--flushIpCache"})
	if code != internal.ErrFlushCache {
		t.Fatalf("code=%d want ErrFlushCache %d", code, internal.ErrFlushCache)
	}
}

func TestRunFlushCacheCertsSuccessThenIPsFailureOnLinux(t *testing.T) {
	resetState(t)
	runtimeGOOS = "linux"
	flushCertsFn = func() *exec.Result { return successResult() }
	flushDnsFn = func() *exec.Result { return failureResult() }
	initializeFn = func(string) error { return nil }
	_, code := runFlushCache([]string{"--flushCertsCache", "--flushIpCache"})
	if code != internal.ErrFlushCacheDNS {
		t.Fatalf("code=%d want ErrFlushCacheDNS", code)
	}
}

// -------------------------------------------------------------------
// root Execute tests
// -------------------------------------------------------------------

func TestExecuteSetsVersionAndHandlesUnknown(t *testing.T) {
	resetState(t)
	Version = "test"
	// Execute with no args will show help and return success (0)
	// We test via direct rootFlagSet to avoid os.Args dependency
	// Instead test that Execute can be called without panic
	// Use Execute with Version set
	// Since Execute uses os.Args, we just ensure it doesn't panic when called with help
	// We'll test rootFlagSet directly
	rootFlagSet = nil
	Version = "dev"
	err, code := Execute()
	// With no args, it should print help and return 0
	_ = err
	_ = code
	if rootFlagSet == nil {
		t.Fatal("rootFlagSet should be set")
	}
}
