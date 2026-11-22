package ops

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
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

// runCertSetupFn performs the config setup that writes the certificate file. It
// is a variable so a test can see what would be written without running the
// config binary, the same reason AddSigKey takes its setup as a variable.
var runCertSetupFn = func(options *executor.ConfigSetupOptions) *exec.Result { return options.RunSetUp() }

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

// AddApiCert trusts the certificate of a standalone API server, or leaves it in
// a file for a custom launcher. It is AddCert's counterpart for a server that
// publishes no announcement: the post-install check is the API's own liveness
// probe, answered over TLS the trust store now accepts.
func (c *Config) AddApiCert(gameId string, apiCertificate *x509.Certificate, canAdd string, customCertFile bool, macOsExclusiveMappings bool) (errorCode int) {
	hosts := common.ApiHosts(gameId, macOsExclusiveMappings)
	if !customCertFile {
		// A certificate that is already trusted is left alone, and a run that
		// was not allowed to trust one has to find it trusted already: adding
		// it again would either duplicate it or write to a store that does not
		// exist.
		alreadyTrusted := true
		for _, host := range hosts {
			if status, probeErr := server2.ProbeApiServer(host, false, nil); probeErr != nil || status != http.StatusOK {
				alreadyTrusted = false
				break
			}
		}
		if alreadyTrusted {
			return common.ErrSuccess
		}
		if canAdd == "false" {
			c.report().Fail("The API server certificate must have been trusted manually: canTrustCertificate is false and %s does not accept it.", hosts[0])
			return launcher.ErrConfigCert
		}
	}
	var certMsg string
	var addUserCertData []byte
	var addLocalCertData []byte
	if customCertFile {
		certFile, err := os.CreateTemp("", common.Name+"_api_cert_*.pem")
		if err != nil {
			return launcher.ErrConfigCertAdd
		}
		if err = certFile.Close(); err != nil {
			return launcher.ErrConfigCertAdd
		}
		c.apiCertFilePath, _ = filepath.Abs(certFile.Name())
		addLocalCertData = apiCertificate.Raw
		certMsg = fmt.Sprintf("Saving API server certificate to '%s' file", certFile.Name())
		// Remove the temp file if the setup below fails before consuming it.
		defer func() {
			if errorCode != 0 {
				_ = os.Remove(c.apiCertFilePath)
			}
		}()
	} else {
		certMsg = fmt.Sprintf("Adding API server certificate to %s store", canAdd)
		if runtime.GOOS == "darwin" || canAdd == "user" {
			certMsg += ", accept the dialog"
		}
		if canAdd == "local" {
			addLocalCertData = apiCertificate.Raw
		} else {
			addUserCertData = apiCertificate.Raw
		}
	}
	certMsg += "..."
	c.report().Step("%s", certMsg)
	var err error
	var setupErr error
	if err = commonLogger.FileLogger.Buffer("config_setup_api_CA_store", func(writer io.Writer) {
		cfgSetupOpts := executor.NewConfigSetupOptions()
		cfgSetupOpts.Out = writer
		cfgSetupOpts.OptionsFn = func(options *exec.Options) {
			commonLogger.Println("run config setup for API CA store cert", options.String())
		}
		cfgSetupOpts.GameId = gameId
		cfgSetupOpts.AddUserCertData = addUserCertData
		cfgSetupOpts.AddLocalCertData = addLocalCertData
		cfgSetupOpts.ApiCertFilePath = c.apiCertFilePath
		cfgSetupOpts.AgentEndOnError = !c.RequiresConfigRevert()
		if result := runCertSetupFn(cfgSetupOpts); !result.Success() {
			if customCertFile {
				c.report().Fail("Failed to save API server certificate to file")
			} else {
				c.report().Fail("Failed to trust API server certificate")
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
			status, probeErr := server2.ProbeApiServer(host, false, nil)
			if probeErr != nil || status != http.StatusOK {
				c.report().Fail("%s", host+" must point to the API server and it must be trusted at this point.")
				if probeErr != nil {
					c.report().Fault("Error: %s", probeErr.Error())
				} else {
					c.report().Fault("Status: %d.", status)
				}
				errorCode = launcher.ErrServerConnectSecure
				return
			}
		}
	}
	return
}

// rootCertDomains chooses the hosts the root server's certificate passes may
// look at. They are AllHosts by default; with a separate API server they
// shrink to the root's own half, because an API domain resolves to the
// standalone API server, which publishes neither the announcement this check
// identifies the server by nor a /cacert.pem/root its certificate could be
// compared against, so every check against them would fail no matter how well
// the setup went.
func rootCertDomains(gameId string, macOsExclusiveMappings bool, apiSeparate bool) []string {
	if apiSeparate {
		return common.RootHosts(gameId, macOsExclusiveMappings)
	}
	return common.AllHosts(gameId, macOsExclusiveMappings)
}

func (c *Config) AddCert(gameId string, serverId uuid.UUID, serverCertificate *x509.Certificate, canAdd string, customCertFile bool, macOsExclusiveMappings bool, apiSeparate bool) (errorCode int) {
	hosts := rootCertDomains(gameId, macOsExclusiveMappings, apiSeparate)
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
		if result := runCertSetupFn(cfgSetupOpts); !result.Success() {
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
