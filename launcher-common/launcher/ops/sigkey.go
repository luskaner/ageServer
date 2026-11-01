package ops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/executor"
	"github.com/luskaner/ageServer/launcher-common/launcher/server"
)

// readSigningKeyFn is where the public signing key comes from. It is a variable
// rather than a direct call so a test can answer with a fixture instead of
// raising a server.
var readSigningKeyFn = server.ReadSigningKeyFromServer

// runSigKeySetupFn performs the config setup that writes the key file. It is a
// variable so a test can see what would be written without running the config
// binary, the same reason the session takes every other operation as a variable.
var runSigKeySetupFn = func(options *executor.ConfigSetupOptions) *exec.Result { return options.RunSetUp() }

// AddSigKey saves the server's public signing key to a temporary file so the
// client launcher can be pointed at it, and remembers the file so it is removed
// with the rest of what this run changed.
//
// It is the signing key counterpart of the certificate file branch of AddCert:
// the client is told where the key is with an argument, so the launcher has to
// leave it somewhere the client can read. The file is written by the config
// setup, not here, because the setup is what stores the revert arguments: a file
// this process wrote and the revert did not know about would outlive the run.
func (c *Config) AddSigKey(host string) (exitCode int) {
	signingKey := readSigningKeyFn(host)
	if len(signingKey) == 0 {
		c.report().Fail("Failed to read the signing public key from %s.", host)
		c.report().Detail("The server serves it at /sigkey.der. Make sure it has one: genCert writes it next to the certificates.")
		return launcher.ErrReadSigKey
	}
	sigKeyFile, err := os.CreateTemp("", common.Name+"_sigkey_*.der")
	if err == nil {
		err = sigKeyFile.Close()
	}
	if err != nil {
		c.report().Fail("Failed to create a file for the signing public key.")
		c.report().Fault("Error message: %s", err.Error())
		return launcher.ErrConfigSigKeyAdd
	}
	c.sigKeyFilePath, _ = filepath.Abs(sigKeyFile.Name())
	// Remove the temp file if the setup below fails before consuming it.
	defer func() {
		if exitCode != 0 {
			_ = os.Remove(c.sigKeyFilePath)
			c.sigKeyFilePath = ""
		}
	}()
	c.report().Step("Saving server public signing key to '%s' file...", c.sigKeyFilePath)
	var setupErr error
	if err = commonLogger.FileLogger.Buffer("config_setup_sigkey", func(writer io.Writer) {
		cfgSetupOpts := executor.NewConfigSetupOptions()
		cfgSetupOpts.Out = writer
		cfgSetupOpts.OptionsFn = func(options *exec.Options) {
			commonLogger.Println("run config setup for signing key file", options.String())
		}
		cfgSetupOpts.GameId = c.gameId
		cfgSetupOpts.SigKeyFilePath = c.sigKeyFilePath
		cfgSetupOpts.AddSigKeyData = signingKey
		cfgSetupOpts.AgentEndOnError = !c.RequiresConfigRevert()
		if result := runSigKeySetupFn(cfgSetupOpts); !result.Success() {
			c.report().Fail("Failed to save the signing public key to file")
			exitCode = launcher.ErrConfigSigKeyAdd
			if result.Err != nil {
				c.report().Fault("Error message: %s", result.Err.Error())
				setupErr = result.Err
			}
			if result.ExitCode != common.ErrSuccess {
				c.report().Fault("Exit code: %d.", result.ExitCode)
				setupErr = fmt.Errorf("exit code: %d", result.ExitCode)
			}
		}
	}); err != nil {
		return common.ErrFileLog
	}
	if setupErr != nil {
		return launcher.ErrConfigSigKeyAdd
	}
	// The client's signing key DLL reads the argument from a command line it
	// splits on spaces, so the path it is given has to be one token. Saying so
	// here is better than letting the game silently verify against the key it
	// ships with, which is what an unusable path ends up doing.
	if _, ok := sigKeyArgPath(c.sigKeyFilePath); !ok {
		c.report().Warn("The signing key file path cannot be read by the client: it contains spaces or non-ASCII characters and has no short form.")
		c.report().Detail("It lives in '%s'. Set the TEMP environment variable to a folder whose path has no spaces if the game rejects the server's content.", filepath.Dir(c.sigKeyFilePath))
	}
	return
}
