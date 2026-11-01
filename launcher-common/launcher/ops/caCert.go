package ops

import (
	"crypto/x509"
	"io"

	"github.com/luskaner/ageServer/common/uuid"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/executor"
)

func (c *Config) AddCACertToGame(gameId string, serverId uuid.UUID, serverCertificate *x509.Certificate, gamePath string, caCertPath string, canAddCert bool, macOsExclusiveMappings bool) (exitCode int) {
	c.report().Step("Adding CA certificate to game if needed...")
	caPool, err := common.ReadCertsPool(caCertPath)
	if err != nil {
		c.report().Fail("Could not read game CA certificates: %s", err)
		return launcher.ErrConfigCACertAdd
	}
	var addCert bool
	addCert, exitCode = checkCertMatch(c.report(), serverId, gameId, serverCertificate, common.AllHosts(gameId, macOsExclusiveMappings), caPool, canAddCert)
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
