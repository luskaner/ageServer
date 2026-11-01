package ops

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/luskaner/ageServer/common/uuid"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	server2 "github.com/luskaner/ageServer/common/server"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/launcher/executor"
	"github.com/luskaner/ageServer/launcher-common/launcher/server"
)

func checkCertMatch(r launcher.Reporter, serverId uuid.UUID, gameId string, serverCertificate *x509.Certificate, hosts []string, rootCAs *x509.CertPool, fixable bool) (requiresFixing bool, exitCode int) {
	for _, host := range hosts {
		if err := server2.CheckConnectionFromServer(host, false, rootCAs); err != nil {
			if fixable {
				cert := server.ReadCACertificateFromServer(host)
				if cert == nil {
					r.Fail("%s", "Failed to read certificate from "+host+".")
					r.Fault("Error: %s", err.Error())
					exitCode = launcher.ErrReadCert
					return
				} else if !bytes.Equal(cert.Raw, serverCertificate.Raw) {
					r.Fail("%s", "The certificate for "+host+" does not match the server certificate.")
					r.Fault("Error: %s", err.Error())
					exitCode = launcher.ErrCertMismatch
					return
				}
				requiresFixing = true
			} else {
				r.Fail("%s", host+" must have been trusted manually.")
				r.Fault("Error: %s", err.Error())
				exitCode = launcher.ErrConfigCert
				return
			}
		} else if cert := server.ReadCACertificateFromServer(host); cert == nil || !bytes.Equal(cert.Raw, serverCertificate.Raw) {
			r.Fail("%s", "The certificate for "+host+" does not match the server certificate (or could not be read).")
			exitCode = launcher.ErrCertMismatch
			return
		} else if !server2.LanServerHost(serverId, gameId, host, false, rootCAs) {
			r.Fail("%s", "Something went wrong, "+host+" does not point to a lan server.")
			exitCode = launcher.ErrServerConnectSecure
			return
		}
	}
	return
}

func (c *Config) AddCert(gameId string, serverId uuid.UUID, serverCertificate *x509.Certificate, canAdd string, customCertFile bool, macOsExclusiveMappings bool) (errorCode int) {
	hosts := common.AllHosts(gameId, macOsExclusiveMappings)
	var addCert bool
	if customCertFile {
		addCert = true
	} else {
		addCert, errorCode = checkCertMatch(c.report(), serverId, gameId, serverCertificate, hosts, nil, canAdd != "false")
		if errorCode != common.ErrSuccess {
			return
		}
	}
	if !addCert {
		return
	}
	var certMsg string
	var addUserCertData []byte
	var addLocalCertData []byte
	if customCertFile {
		certFile, err := os.CreateTemp("", common.Name+"_cert_*.pem")
		if err != nil {
			return launcher.ErrConfigCertAdd
		}
		if err = certFile.Close(); err != nil {
			return launcher.ErrConfigCertAdd
		}
		c.certFilePath, _ = filepath.Abs(certFile.Name())
		addLocalCertData = serverCertificate.Raw
		certMsg = fmt.Sprintf("Saving server certificate to '%s' file", certFile.Name())
		// Remove the temp file if the setup below fails before consuming it.
		defer func() {
			if errorCode != 0 {
				_ = os.Remove(c.certFilePath)
			}
		}()
	} else {
		certMsg = fmt.Sprintf("Adding server certificate to %s store", canAdd)
		if runtime.GOOS == "darwin" || canAdd == "user" {
			certMsg += ", accept the dialog"
		}
		if canAdd == "local" {
			addLocalCertData = serverCertificate.Raw
		} else {
			addUserCertData = serverCertificate.Raw
		}
	}
	certMsg += "..."
	c.report().Step("%s", certMsg)
	var err error
	var setupErr error
	if err = commonLogger.FileLogger.Buffer("config_setup_CA_store", func(writer io.Writer) {
		cfgSetupOpts := executor.NewConfigSetupOptions()
		cfgSetupOpts.Out = writer
		cfgSetupOpts.OptionsFn = func(options *exec.Options) {
			commonLogger.Println("run config setup for CA store cert", options.String())
		}
		cfgSetupOpts.GameId = gameId
		cfgSetupOpts.AddUserCertData = addUserCertData
		cfgSetupOpts.AddLocalCertData = addLocalCertData
		cfgSetupOpts.CertFilePath = c.certFilePath
		cfgSetupOpts.AgentEndOnError = !c.RequiresConfigRevert()
		if result := cfgSetupOpts.RunSetUp(); !result.Success() {
			if customCertFile {
				c.report().Fail("Failed to save certificate to file")
			} else {
				c.report().Fail("Failed to trust certificate")
			}
			errorCode = launcher.ErrConfigCertAdd
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
		return launcher.ErrConfigCertAdd
	}
	if !customCertFile {
		for _, host := range hosts {
			if err = server2.CheckConnectionFromServer(host, false, nil); err != nil {
				c.report().Fail("%s", host+" must have been trusted automatically at this point.")
				c.report().Fault("Error: %s", err.Error())
				errorCode = launcher.ErrServerConnectSecure
				return
			} else if !server2.LanServerHost(serverId, gameId, host, false, nil) {
				c.report().Fail("%s", "Something went wrong, "+host+" either points to the original server or there is a certificate issue.")
				errorCode = launcher.ErrTrustCert
				return
			}
		}
	}
	return
}
