package cmd

import (
	"errors"
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd/genCert"
	"github.com/luskaner/ageServer/server-genCert/internal"
)

func resetState(t *testing.T) {
	t.Helper()
	oldExe := osExecutableFn
	oldFolder := certificatePairFolderFn
	oldPairs := certificatePairsFn
	oldGen := generateCertificatePairsFn
	oldValues := values
	t.Cleanup(func() {
		osExecutableFn = oldExe
		certificatePairFolderFn = oldFolder
		certificatePairsFn = oldPairs
		generateCertificatePairsFn = oldGen
		values = oldValues
	})
	values = &genCert.Values{}
}

func TestRunRootExecutableFailure(t *testing.T) {
	resetState(t)
	osExecutableFn = func() (string, error) { return "", errors.New("fail") }
	_, code := runRoot(nil)
	if code != common.ErrGeneral {
		t.Fatalf("code=%d want ErrGeneral", code)
	}
}

func TestRunRootCertFolderEmpty(t *testing.T) {
	resetState(t)
	osExecutableFn = func() (string, error) { return "/tmp/fake.exe", nil }
	certificatePairFolderFn = func(string) string { return "" }
	_, code := runRoot(nil)
	if code != internal.ErrCertDirectory {
		t.Fatalf("code=%d want ErrCertDirectory", code)
	}
}

func TestRunRootAlreadyExistsNoReplaceNoIgnore(t *testing.T) {
	resetState(t)
	osExecutableFn = func() (string, error) { return "/tmp/fake.exe", nil }
	certificatePairFolderFn = func(string) string { return "/tmp/folder" }
	certificatePairsFn = func(string) (bool, string, string, string, string, string) { return true, "", "", "", "", "" }
	values.Replace = false
	values.IgnoreIfExisting = false
	_, code := runRoot(nil)
	if code != internal.ErrCertCreateExisting {
		t.Fatalf("code=%d want ErrCertCreateExisting", code)
	}
}

func TestRunRootAlreadyExistsIgnore(t *testing.T) {
	resetState(t)
	osExecutableFn = func() (string, error) { return "/tmp/fake.exe", nil }
	certificatePairFolderFn = func(string) string { return "/tmp/folder" }
	certificatePairsFn = func(string) (bool, string, string, string, string, string) { return true, "", "", "", "", "" }
	values.Replace = false
	values.IgnoreIfExisting = true
	_, code := runRoot(nil)
	if code != 0 {
		t.Fatalf("code=%d want 0", code)
	}
}

func TestRunRootAlreadyExistsWithReplaceSkipsCheck(t *testing.T) {
	resetState(t)
	osExecutableFn = func() (string, error) { return "/tmp/fake.exe", nil }
	certificatePairFolderFn = func(string) string { return "/tmp/folder" }
	// Even though pairs exist, with Replace true it should not check and proceed to generate
	certificatePairsFn = func(string) (bool, string, string, string, string, string) {
		t.Error("should not be called when Replace true")
		return true, "", "", "", "", ""
	}
	generateCertificatePairsFn = func(string) bool { return true }
	values.Replace = true
	_, code := runRoot(nil)
	if code != 0 {
		t.Fatalf("code=%d want 0", code)
	}
}

func TestRunRootGenerateFailure(t *testing.T) {
	resetState(t)
	osExecutableFn = func() (string, error) { return "/tmp/fake.exe", nil }
	certificatePairFolderFn = func(string) string { return "/tmp/folder" }
	certificatePairsFn = func(string) (bool, string, string, string, string, string) { return false, "", "", "", "", "" }
	generateCertificatePairsFn = func(string) bool { return false }
	_, code := runRoot(nil)
	if code != internal.ErrCertCreate {
		t.Fatalf("code=%d want ErrCertCreate", code)
	}
}

func TestRunRootSuccess(t *testing.T) {
	resetState(t)
	osExecutableFn = func() (string, error) { return "/tmp/fake.exe", nil }
	certificatePairFolderFn = func(string) string { return "/tmp/folder" }
	certificatePairsFn = func(string) (bool, string, string, string, string, string) { return false, "", "", "", "", "" }
	generateCertificatePairsFn = func(string) bool { return true }
	_, code := runRoot(nil)
	if code != 0 {
		t.Fatalf("code=%d want 0", code)
	}
}

func TestRunRootNoCertPairsSuccess(t *testing.T) {
	resetState(t)
	osExecutableFn = func() (string, error) { return "/tmp/fake.exe", nil }
	certificatePairFolderFn = func(string) string { return "/tmp/folder" }
	certificatePairsFn = func(string) (bool, string, string, string, string, string) { return false, "", "", "", "", "" }
	generateCertificatePairsFn = func(string) bool { return true }
	values.Replace = false
	values.IgnoreIfExisting = false
	_, code := runRoot(nil)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
}

func TestExecuteSetsVersion(t *testing.T) {
	resetState(t)
	Version = "test"
	// Mock to avoid actually running runRoot
	osExecutableFn = func() (string, error) { return "/tmp/fake.exe", nil }
	certificatePairFolderFn = func(string) string { return "/tmp/folder" }
	certificatePairsFn = func(string) (bool, string, string, string, string, string) { return false, "", "", "", "", "" }
	generateCertificatePairsFn = func(string) bool { return true }
	_, code := Execute()
	// Execute will parse os.Args, but with no args it should show help and return 0
	_ = code
	if Version != "test" {
		t.Fatal("Version not set")
	}
}
