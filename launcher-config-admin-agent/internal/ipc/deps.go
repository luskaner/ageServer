package ipc

import (
	"crypto/x509"
	"io"

	"github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/executor"
)

// Injectable vars for testing
var (
	runSetUpFn  = executor.RunSetUp
	runRevertFn = executor.RunRevert
	// buffer wraps FileLogger.Buffer with nil handling like in ipc.go
	bufferFn = func(name string, fn func(writer io.Writer)) error {
		if commonLogger.FileLogger == nil {
			fn(nil)
			return nil
		}
		return commonLogger.FileLogger.Buffer(name, fn)
	}
	parseCertFn = x509.ParseCertificate
	// For StartServer mocking
	setupServerFn  = SetupServer
	revertServerFn = RevertServer
)

var _ = parseCertFn
var _ = bufferFn
var _ = runSetUpFn
var _ = runRevertFn
