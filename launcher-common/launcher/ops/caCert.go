package ops

import (
	"bytes"
	"crypto/x509"
	"io"

	"github.com/luskaner/ageServer/common/uuid"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/executor"
)

func (c *Config) AddCACertToGame(gameId string, serverId uuid.UUID, serverCertificate *x509.Certificate, gamePath string, caCertPath string, canAddCert bool, macOsExclusiveMappings bool, apiSeparate bool) (exitCode int) {
	c.report().Step("Adding CA certificate to game if needed...")
	caPool, err := common.ReadCertsPool(caCertPath)
	if err != nil {
		c.report().Fail("Could not read game CA certificates: %s", err)
		return launcher.ErrConfigCACertAdd
	}
	hosts := rootCertDomains(gameId, macOsExclusiveMappings, apiSeparate)
	var addCert bool
	addCert, exitCode = checkCertMatch(c.report(), serverId, gameId, serverCertificate, hosts, caPool, canAddCert)
	if !addCert || exitCode != common.ErrSuccess {
		return
	}
	if err = commonLogger.FileLogger.Buffer("config_setup_CA_game", func(writer io.Writer) {
		cfgSetupOpts := executor.NewConfigSetupOptions()
		cfgSetupOpts.Out = writer
		cfgSetupOpts.OptionsFn = func(options *exec.Options) {
			commonLogger.Println("run config setup for CA game cert", options.String())
		}
		cfgSetupOpts.GameId = gameId
		cfgSetupOpts.GamePath = gamePath
		cfgSetupOpts.AddCACertData = serverCertificate.Raw
		cfgSetupOpts.AgentEndOnError = !c.RequiresConfigRevert()
		if result := cfgSetupOpts.RunSetUp(); !result.Success() {
			c.report().Fail("Failed to save CA certificate to game")
			exitCode = launcher.ErrConfigCACertAdd
			if result.Err != nil {
				c.report().Fault("Error message: %s", result.Err.Error())
			}
			if result.ExitCode != common.ErrSuccess {
				c.report().Fault("Exit code: %d.", result.ExitCode)
			}
		}
	}); err != nil {
		c.report().Fail("Error message: %s", err.Error())
		return common.ErrFileLog
	}
	return
}

// AddApiCACertToGame adds the standalone API server's certificate to the
// game's CA store, the counterpart of AddCACertToGame for the API domains: a
// game such as AoE II: DE validates what it connects to against its own
// bundle, so the API certificate has to land there too.
//
// Whether it is already there is decided by content and not by connecting.
// Connecting would resolve the API domains through the system resolver, which
// with a {HostFilePath} client knows nothing about the session's mapping at
// this point — the DNS cache only holds the root server's half — so a
// connection check would either look at the official server or fail, and the
// certificate would be appended again on every run.
func (c *Config) AddApiCACertToGame(gameId string, apiCertificate *x509.Certificate, gamePath string, caCertPath string, canAddCert bool, macOsExclusiveMappings bool) (exitCode int) {
	c.report().Step("Adding API server CA certificate to game if needed...")
	_, _, caCerts, err := common.ReadFromFile(caCertPath)
	if err != nil {
		c.report().Fail("Could not read game CA certificates: %s", err)
		return launcher.ErrConfigCACertAdd
	}
	for _, cert := range caCerts {
		if bytes.Equal(cert.Raw, apiCertificate.Raw) {
			return common.ErrSuccess
		}
	}
	if !canAddCert {
		c.report().Fail("%s", "The API server certificate must have been added to the game's store manually.")
		return launcher.ErrConfigCert
	}
	if err = commonLogger.FileLogger.Buffer("config_setup_CA_game_api", func(writer io.Writer) {
		cfgSetupOpts := executor.NewConfigSetupOptions()
		cfgSetupOpts.Out = writer
		cfgSetupOpts.OptionsFn = func(options *exec.Options) {
			commonLogger.Println("run config setup for CA game API cert", options.String())
		}
		cfgSetupOpts.GameId = gameId
		cfgSetupOpts.GamePath = gamePath
		cfgSetupOpts.AddCACertData = apiCertificate.Raw
		cfgSetupOpts.AgentEndOnError = !c.RequiresConfigRevert()
		if result := runCertSetupFn(cfgSetupOpts); !result.Success() {
			c.report().Fail("Failed to save API server CA certificate to game")
			exitCode = launcher.ErrConfigCACertAdd
			if result.Err != nil {
				c.report().Fault("Error message: %s", result.Err.Error())
			}
			if result.ExitCode != common.ErrSuccess {
				c.report().Fault("Exit code: %d.", result.ExitCode)
			}
		}
	}); err != nil {
		c.report().Fail("Error message: %s", err.Error())
		return common.ErrFileLog
	}
	return
}
