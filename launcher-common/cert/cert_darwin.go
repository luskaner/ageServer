package cert

import (
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/certStore"
	"github.com/luskaner/ageServer/common/executor/exec"
)

func TrustCertificates(userStore bool, certs []*x509.Certificate) error {
	keychain, updateAsAdmin, err := certStore.KeychainPath(userStore)
	if err != nil {
		return err
	}
	for _, cert := range certs {
		certFile, err := os.CreateTemp("", common.Name+"_cert_*.pem")
		if err != nil {
			return err
		}
		certPath := certFile.Name()
		if err = common.WriteAsPem(cert.Raw, certFile); err != nil {
			_ = certFile.Close()
			_ = os.Remove(certPath)
			return err
		}
		if err = certFile.Close(); err != nil {
			_ = os.Remove(certPath)
			return err
		}
		args := []string{"add-trusted-cert"}
		if !userStore {
			args = append(args, "-d")
		}
		args = append(args, "-k", keychain, certPath)
		args = append(args, "&&", "rm", "-f", certPath)
		err = certStore.RunCommandWithoutOutput(updateAsAdmin, func(options *exec.Options) {
			if !common.Interactive() {
				options.ShowWindow = true
			}
			options.Shell = true
		}, args...)
		if err != nil {
			return err
		}
	}
	if len(certs) > 0 {
		if result := FlushCerts(); !result.Success() {
			if result.Err != nil {
				err = result.Err
			} else {
				err = fmt.Errorf("error flushing certs, exit code %d", result.ExitCode)
			}
			return err
		}
	}
	return nil
}

func UntrustCertificates(userStore bool) (certs []*x509.Certificate, err error) {
	var existing []*x509.Certificate
	existing, err = EnumCertificates(userStore)
	if err != nil {
		return
	}
	if len(existing) == 0 {
		return
	}
	keychain, updateAsAdmin, err := certStore.KeychainPath(userStore)
	if err != nil {
		return nil, err
	}
	certs = make([]*x509.Certificate, 0, len(existing))
	for _, cert := range existing {
		match := false
		for _, org := range cert.Subject.Organization {
			if org == common.CertSubjectOrganization {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		fingerprint := sha1.Sum(cert.Raw)
		fingerprintHex := strings.ToUpper(hex.EncodeToString(fingerprint[:]))
		err = certStore.RunCommandWithoutOutput(updateAsAdmin, nil, "delete-certificate", "-Z", fingerprintHex, keychain)
		if err != nil {
			return
		}
		certs = append(certs, cert)
	}
	if len(certs) > 0 {
		if result := FlushCerts(); !result.Success() {
			if result.Err != nil {
				err = result.Err
			} else {
				err = fmt.Errorf("error flushing certs, exit code %d", result.ExitCode)
			}
			return
		}
	}
	return
}

func EnumCertificates(userStore bool) (certs []*x509.Certificate, err error) {
	var keychain string
	keychain, _, err = certStore.KeychainPath(userStore)
	if err != nil {
		return
	}
	var output string
	output, err = certStore.RunCommandWithOutput(false, "find-certificate", "-a", "-p", keychain)
	if err != nil {
		return
	}
	data := []byte(output)
	_, _, certs, err = common.ReadFromData(data)
	return
}
