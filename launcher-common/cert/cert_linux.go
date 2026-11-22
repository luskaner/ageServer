package cert

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hairyhenderson/go-which"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
)

var updateStoreBinaries = []string{
	// Debian, OpenSUSE
	"update-ca-certificates",
	// Fedora, Arch
	"update-ca-trust",
}

var certStorePaths = []string{
	// Arch
	"/etc/ca-certificates/trust-source/anchors",
	// Debian
	"/usr/local/share/ca-certificates",
	// Fedora
	"/etc/pki/ca-trust/source/anchors",
	// OpenSUSE
	"/etc/pki/trust/anchors",
}

var certStoreBundlePaths = []string{
	// Debian, Arch
	"/etc/ssl/certs/ca-certificates.crt",
	// Fedora
	"/etc/pki/tls/certs/ca-bundle.crt",
	// OpenSUSE
	"/etc/pki/tls/certs/ca-bundle.pem",
	// Other
	"/etc/ssl/ca-bundle.pem",
}

// flushCertsFn runs the store update. It is a variable so a test that writes a
// store file does not have to run the update binary, which needs root through
// pkexec and is not there on a CI runner.
var flushCertsFn = FlushCerts

func FlushCerts() (result *exec.Result) {
	binary := which.Which(updateStoreBinaries...)
	if binary == "" {
		return &exec.Result{Err: fmt.Errorf("update store binary not found")}
	}
	return exec.Options{
		File:        binary,
		SpecialFile: true,
		AsAdmin:     true,
		Wait:        true,
		ExitCode:    true,
	}.Exec()
}

func getCertPath() (err error, certPath string) {
	var stat os.FileInfo
	var foundPath string
	for _, dir := range certStorePaths {
		if stat, err = os.Stat(dir); err == nil && stat.IsDir() {
			foundPath = dir
			break
		}
	}
	if foundPath == "" {
		err = fmt.Errorf("cert store not found")
		return
	}
	certPath = filepath.Join(foundPath, fmt.Sprintf("%s.crt", common.Name))
	return
}

// readStoreCertificates parses every PEM certificate the store file holds.
// The file is ours and is a bundle: a session trusts the API server's
// certificate first and the root server's afterwards, so the store has to
// hold them both.
func readStoreCertificates(certPath string) (certs []*x509.Certificate, err error) {
	var certBytes []byte
	certBytes, err = os.ReadFile(certPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return
	}
	for len(certBytes) > 0 {
		var block *pem.Block
		block, certBytes = pem.Decode(certBytes)
		if block == nil {
			break
		}
		var cert *x509.Certificate
		if cert, err = x509.ParseCertificate(block.Bytes); err != nil {
			return
		}
		certs = append(certs, cert)
	}
	return
}

// mergeCertificates keeps the store's order and appends what is not already
// in it, so repeated passes and repeated runs do not grow the file.
func mergeCertificates(existing []*x509.Certificate, added []*x509.Certificate) []*x509.Certificate {
	merged := existing
	for _, cert := range added {
		duplicate := false
		for _, already := range merged {
			if bytes.Equal(already.Raw, cert.Raw) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			merged = append(merged, cert)
		}
	}
	return merged
}

// writeStoreFile atomically replaces the store file with the merged bundle,
// through the same temp-file dance as before: a rename across filesystems is
// retried by streaming through a temp file next to the destination.
func writeStoreFile(certPath string, certs []*x509.Certificate) (err error) {
	var certFile *os.File
	certFile, err = os.CreateTemp("", "*")
	if err != nil {
		return err
	}
	for _, cert := range certs {
		if err = common.WriteAsPem(cert.Raw, certFile); err != nil {
			_ = certFile.Close()
			_ = os.Remove(certFile.Name())
			return err
		}
	}
	if err = certFile.Close(); err != nil {
		_ = os.Remove(certFile.Name())
		return err
	}

	err = os.Rename(certFile.Name(), certPath)
	if err != nil {
		// Likely certFile does not share filesystem with certPath
		var certFileTmp *os.File
		certFileTmp, err = os.CreateTemp(filepath.Dir(certPath), ".*")
		if err != nil {
			_ = os.Remove(certFile.Name())
			return err
		}

		var newCertFile *os.File
		newCertFile, err = os.Open(certFile.Name())
		if err != nil {
			_ = certFileTmp.Close()
			_ = os.Remove(certFile.Name())
			_ = os.Remove(certFileTmp.Name())
			return err
		}

		_, err = io.Copy(certFileTmp, newCertFile)
		_ = newCertFile.Close()
		_ = os.Remove(newCertFile.Name())
		if err != nil {
			_ = certFileTmp.Close()
			_ = os.Remove(certFileTmp.Name())
			_ = os.Remove(certFile.Name())
			return err
		}

		err = certFileTmp.Close()
		if err != nil {
			_ = os.Remove(certFileTmp.Name())
			_ = os.Remove(certFile.Name())
			return err
		}

		err = os.Rename(certFileTmp.Name(), certPath)
		if err != nil {
			_ = os.Remove(certFileTmp.Name())
			_ = os.Remove(certFile.Name())
			return err
		}
		_ = os.Remove(certFile.Name())
	}

	return os.Chmod(certPath, 0644)
}

// TrustCertificates adds the certificates to the system's trusted store: they
// all land in one bundle file that starts from what the store already holds,
// so the session's root pass does not replace the API pass's certificate and
// a repeated pass does not grow the file.
func TrustCertificates(_ bool, certs []*x509.Certificate) error {
	err, certPath := getCertPath()
	if err != nil {
		return err
	}

	var existing []*x509.Certificate
	if existing, err = readStoreCertificates(certPath); err != nil {
		return err
	}
	merged := mergeCertificates(existing, certs)
	if len(merged) == 0 {
		return nil
	}

	if err = writeStoreFile(certPath, merged); err != nil {
		return err
	}

	return flushCertsFn().Err
}

// UntrustCertificates removes the store file whole and returns every
// certificate it held, so a restore can put the bundle back as it was.
func UntrustCertificates(_ bool) (certs []*x509.Certificate, err error) {
	var certPath string
	err, certPath = getCertPath()
	if err != nil {
		return
	}

	if _, err = os.Stat(certPath); os.IsNotExist(err) {
		return
	}

	if certs, err = readStoreCertificates(certPath); err != nil {
		return
	}

	if err = os.Remove(certPath); err != nil {
		return
	}

	if err = flushCertsFn().Err; err != nil {
		// The update never ran, so the caller can put the file back whole.
		return certs, err
	}

	// The certificates are returned even on success: a revert interrupted after
	// this point has to be able to trust them again, as on Windows and macOS.
	return
}

func EnumCertificates(_ bool) (certs []*x509.Certificate, err error) {
	for _, path := range certStoreBundlePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		certs = []*x509.Certificate{}
		for len(data) > 0 {
			var block *pem.Block
			block, data = pem.Decode(data)
			if block == nil {
				break
			}
			if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
				continue
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				continue
			}
			certs = append(certs, cert)
		}
		return certs, nil
	}

	return nil, fmt.Errorf("no cert bundle found")
}
