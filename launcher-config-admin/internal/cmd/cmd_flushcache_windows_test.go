package cmd

import (
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/launcher-config-admin/internal"
)

// Split out of cmd_test.go: these two assert that the certificate flush is
// skipped, which only happens on Windows. Run anywhere else they failed, one of
// them by design. Moved verbatim; only the location changed.
func TestRunFlushCacheCertsSkippedOnWindows(t *testing.T) {
	resetState(t)
	// On Windows, Certs flush is skipped, so even if we pass --flushCertsCache, it should succeed without calling flushCertsFn
	called := false
	flushCertsFn = func() *exec.Result { called = true; return failureResult() }
	flushDnsFn = func() *exec.Result { return successResult() }
	initializeFn = func(string) error { return nil }
	_, code := runFlushCache([]string{"--flushCertsCache"})
	// On windows, this should be success and not call flushCerts
	if called {
		t.Fatal("flushCerts should not be called on windows")
	}
	if code != common.ErrSuccess {
		t.Fatalf("code=%d want 0", code)
	}
}

func TestRunFlushCacheBothIPsAndCertsFailure(t *testing.T) {
	resetState(t)
	// On windows, certs part is skipped, so only IPs failure matters
	flushDnsFn = func() *exec.Result { return failureResult() }
	initializeFn = func(string) error { return nil }
	_, code := runFlushCache([]string{"--flushIpCache", "--flushCertsCache"})
	// Since certs skipped on windows, this is just IPs failure => DNS error
	if code != internal.ErrFlushCacheDNS {
		t.Fatalf("code=%d want ErrFlushCacheDNS", code)
	}
}
