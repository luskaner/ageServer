package ipc

import (
	"crypto/x509"
	"encoding/gob"
	"errors"
	"io"
	"net"
	"strings"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/launcher-common/cmdlog"
	"github.com/luskaner/ageServer/launcher-common/ipc"
	"github.com/luskaner/ageServer/launcher-config-admin-agent/internal"
	"golang.org/x/net/idna"
)

// mappedIps and addedCert remember what this agent process did, so a Revert
// only has to be told which Kinds to undo and a failed run leaves nothing
// behind. They are deliberately not guards: one session maps hosts and adds
// certificates more than once (an API pass and the server's own pass when a
// standalone API server is configured), so a repeat Setup has to succeed.
// Idempotency lives where the change happens — the hosts file keeps our
// marker, the certificate store keeps our subject — not in a one-shot flag.
var mappedIps = false
var addedCert = false

func handleClient(logRoot string, c net.Conn) (exit bool) {
	exit = false
	decoder := gob.NewDecoder(c)
	encoder := gob.NewEncoder(c)
	var action byte
	var err error

	for !exit {
		if err = decoder.Decode(&action); err != nil {
			if errors.Is(err, io.EOF) {
				cmdlog.Step("Closing connection...")
				return
			}
			cmdlog.Fail("Could not decode action: %s", err)
			str := "-> ErrDecode: "
			if err = encoder.Encode(internal.ErrDecode); err != nil {
				str += err.Error()
			} else {
				str += "OK"
			}
			cmdlog.Fail("%s", str)
			// The gob stream state is no longer trustworthy after a failed
			// decode: continuing would loop on the same garbage forever.
			return
		}

		var exitCode = internal.ErrNonExistingAction

		switch action {
		case ipc.Revert:
			str := "<- Revert: "
			if err = encoder.Encode(common.ErrSuccess); err != nil {
				str += err.Error()
				cmdlog.Fail("%s", str)
			} else {
				cmdlog.Step("%sOK", str)
			}
			exitCode = handleRevert(logRoot, decoder)
		case ipc.Setup:
			str := "<- Setup: "
			if err = encoder.Encode(common.ErrSuccess); err != nil {
				str += err.Error()
				cmdlog.Fail("%s", str)
			} else {
				cmdlog.Step("%sOK", str)
			}
			exitCode = handleSetUp(logRoot, decoder)
		case ipc.Exit:
			str := "<- Exit: "
			// Acknowledge before tearing the connection down. Our clients run
			// non-elevated and so cannot terminate this elevated process, which
			// makes this handshake the only thing able to stop us. Without an
			// ack the client cannot tell "still busy" from "never received the
			// command", and used to guess with a short timed poll.
			if err = encoder.Encode(common.ErrSuccess); err != nil {
				str += err.Error()
				exitCode = internal.ErrConnectionClosing
			} else {
				str += "OK"
			}
			if closeErr := c.Close(); closeErr != nil {
				cmdlog.Fail("%s", str)
				cmdlog.Fault("Could not close connection: %s", closeErr)
				if exitCode == common.ErrSuccess {
					exitCode = internal.ErrConnectionClosing
				}
			} else {
				cmdlog.Step("%s", str)
			}
			exit = true
		}

		// The Exit branch already replied and closed the connection; encoding
		// again would write to a closed handle.
		if !exit {
			_ = encoder.Encode(exitCode)
		}
	}

	return
}

func checkCertificateValidity(cert *x509.Certificate, gameId string) bool {
	if cert == nil {
		return false
	}
	// Security checks
	// Disallow any domain or IP in CN
	if strings.Contains(cert.Subject.CommonName, "*") {
		return false
	}
	if _, err := idna.Lookup.ToASCII(cert.Subject.CommonName); err == nil {
		return false
	}
	if parsedIP := net.ParseIP(cert.Subject.CommonName); parsedIP != nil {
		return false
	}
	if !cert.IsCA {
		return false
	}
	expectedKeyUsage := x509.KeyUsageCertSign
	expectedExtKeyUsages := mapset.NewSet[x509.ExtKeyUsage]()
	if common.SelfSignedCertGame(gameId) {
		if !cert.MaxPathLenZero {
			// A legacy title may be served from a modern listener (the two
			// modes exist precisely so one process can serve both classes).
			// Accept the modern CA profile with a warning; the pinning of the
			// bytes, the CN and IsCA above are not relaxed.
			if modernCaProfile(cert) {
				cmdlog.Warn("Game %s is a legacy title but the certificate is the modern CA profile; accepting it with a warning", gameId)
				return true
			}
			return false
		}
		selfSignedCertDomains := mapset.NewSet[string](common.SelfSignedCertDomains...)
		certDNSNames := mapset.NewSet[string](cert.DNSNames...)
		if !selfSignedCertDomains.Equal(certDNSNames) {
			return false
		}
		expectedKeyUsage |= x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
		expectedExtKeyUsages.Append(x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth)
	}
	if expectedKeyUsage != cert.KeyUsage {
		return false
	}
	if !expectedExtKeyUsages.Equal(mapset.NewSet[x509.ExtKeyUsage](cert.ExtKeyUsage...)) {
		return false
	}
	return true
}

// modernCaProfile is the CA half of the modern pair as server-genCert creates
// it: a CA with only CertSign and no extended usage or SANs. It is deliberately
// narrower than the legacy profile, not a general "any CA" escape hatch.
func modernCaProfile(cert *x509.Certificate) bool {
	return cert.IsCA &&
		cert.KeyUsage == x509.KeyUsageCertSign &&
		len(cert.ExtKeyUsage) == 0 &&
		len(cert.DNSNames) == 0 &&
		len(cert.IPAddresses) == 0
}

func handleSetUp(logRoot string, decoder *gob.Decoder) int {
	var msg ipc.SetupCommand
	cmdlog.Step("<- SetupCommand")
	if err := decoder.Decode(&msg); err != nil {
		cmdlog.Fail("Could not decode command: %s", err)
		return internal.ErrDecode
	}
	cmdlog.Printf("<- %v\n", msg)
	if !game.SupportedGames.ContainsOne(msg.GameId) {
		cmdlog.Fail("Game is not supported")
		return internal.ErrGameNotSupported
	}
	if msg.Api && msg.RootOnly {
		cmdlog.Fail("api and rootOnly mappings cannot both be set")
		return internal.ErrHostSetConflict
	}
	var cert *x509.Certificate
	if msg.Certificate != nil {
		str := "Parsing certificate: "
		var err error
		cert, err = parseCertFn(msg.Certificate)
		if err != nil || !checkCertificateValidity(cert, msg.GameId) {
			if err != nil {
				str += err.Error()
			} else {
				str += "invalid"
			}
			return internal.ErrCertInvalid
		}

		str += "OK"
		cmdlog.Println(str)
	}
	var suffix string
	if cert != nil {
		suffix = "_cert"
	} else {
		suffix = "_hosts"
	}
	var result *exec.Result
	if buffErr := bufferFn("config-admin_setup"+suffix, func(writer io.Writer) {
		result = runSetUpFn(msg.GameId, msg.IP, msg.MacOsExclusiveMappings, msg.CanUseInternet, msg.Api, msg.RootOnly, cert, logRoot, writer, func(options *exec.Options) {
			if writer != nil {
				cmdlog.Println("run config admin setup", options.String())
			}
		})
	}); buffErr != nil {
		return common.ErrFileLog
	}
	if result.Success() {
		mappedIps = mappedIps || len(msg.IP) > 0
		addedCert = addedCert || cert != nil
	}
	return result.ExitCode
}

func handleRevert(logRoot string, decoder *gob.Decoder) int {
	var msg ipc.RevertCommand
	cmdlog.Step("<- RevertCommand")
	if err := decoder.Decode(&msg); err != nil {
		cmdlog.Fail("Could not decode command: %s", err)
		return internal.ErrDecode
	}
	cmdlog.Printf("<- %v\n", msg)
	revertIps := msg.IPs && mappedIps
	revertCert := msg.Certificate && addedCert
	if !revertIps && !revertCert {
		cmdlog.Info("Everything is already reverted.")
		return common.ErrSuccess
	}
	var result *exec.Result
	if buffErr := bufferFn("config-admin_revert", func(writer io.Writer) {
		result = runRevertFn(revertIps, revertCert, true, logRoot, writer, func(options *exec.Options) {
			if writer != nil {
				cmdlog.Println("run config admin revert", options.String())
			}
		})
	}); buffErr != nil {
		return common.ErrFileLog
	}
	if result.Success() {
		mappedIps = mappedIps && !revertIps
		addedCert = addedCert && !revertCert
	}
	return result.ExitCode
}

// Listen creates the named pipe without accepting anything on it yet.
//
// It is separate from Serve so the caller can make the pipe exist before it does
// any slow work. A client only needs the pipe to be present to consider the
// agent up; making that depend on a cache flush meant a slow machine could not
// be reached in time, and the caller's answer to that was to kill an agent that
// was merely still starting.
func Listen() (net.Listener, error) {
	l, err := setupServerFn()
	if err != nil {
		cmdlog.Fail("Could not listen to IPC: %s", err)
		return nil, err
	}
	return l, nil
}

// Serve accepts connections until one asks the agent to exit.
func Serve(logRoot string, l net.Listener) (exitCode int) {
	defer func(l net.Listener) {
		_ = l.Close()
		revertServerFn()
	}(l)

	for {
		cmdlog.Step("Waiting for connection...")
		conn, err := l.Accept()
		if err != nil {
			cmdlog.Fail("Could not accept connection: %s", err)
			continue
		}
		cmdlog.Ok("Accepted connection: %s", conn.RemoteAddr())
		if handleClient(logRoot, conn) {
			break
		}
	}
	return
}

func StartServer(logRoot string) (exitCode int) {
	l, err := Listen()
	if err != nil {
		exitCode = internal.ErrListen
		return
	}
	return Serve(logRoot, l)
}
