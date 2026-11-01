package common

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"

	"github.com/luskaner/ageServer/common/paths"
)

func ReadCertsPool(path string) (pool *x509.CertPool, err error) {
	var caCerts []*x509.Certificate
	_, _, caCerts, err = ReadFromFile(path)
	if err != nil {
		return
	}
	pool = x509.NewCertPool()
	for _, caCert := range caCerts {
		pool.AddCert(caCert)
	}
	return
}

func BytesToCertificate(data []byte) *x509.Certificate {
	cert, _ := x509.ParseCertificate(data)
	return cert
}

func WriteAsPem(data []byte, file *os.File) error {
	pemData := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: data,
	})
	_, err := file.Write(pemData)
	return err
}

func ReadFromFile(filePath string) (keys []string, keyToIndex map[string]int, values []*x509.Certificate, err error) {
	var pemData []byte
	pemData, err = os.ReadFile(filePath)
	if err != nil {
		return
	}
	return ReadFromData(pemData)
}

func ReadFromData(data []byte) (keys []string, keyToIndex map[string]int, values []*x509.Certificate, err error) {
	keys = make([]string, 0)
	values = make([]*x509.Certificate, 0)
	keyToIndex = make(map[string]int)
	var cert *x509.Certificate
	for {
		block, pemData := pem.Decode(data)
		if block == nil {
			break
		}

		data = pemData

		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, err = x509.ParseCertificate(block.Bytes)
		if err != nil {
			return
		}

		hash := sha256.Sum256(cert.Raw)
		fingerprint := hex.EncodeToString(hash[:])

		keys = append(keys, fingerprint)
		values = append(values, cert)
		keyToIndex[fingerprint] = len(keys) - 1

		if len(data) == 0 {
			break
		}
	}
	return
}

func certificatePairFolderPath(executablePath string) string {
	if executablePath == "" {
		return ""
	}
	parentDir := filepath.Dir(executablePath)
	if parentDir == "" {
		return ""
	}
	return filepath.Join(parentDir, paths.ResourcesDir, "certificates")
}

func CertificatePairFolder(executablePath string) string {
	folder := certificatePairFolderPath(executablePath)
	if folder == "" {
		return ""
	}
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		if os.MkdirAll(folder, 0755) != nil {
			return ""
		}
	}
	return folder
}

func CertificatePairs(parentDir string) (ok bool, cert string, key string, caCert string, selfSignedCert string, selfSignedKey string) {
	if parentDir == "" {
		return
	}
	cert = filepath.Join(parentDir, Cert)
	if _, err := os.Stat(cert); os.IsNotExist(err) {
		return
	}
	key = filepath.Join(parentDir, Key)
	if _, err := os.Stat(filepath.Join(parentDir, Key)); os.IsNotExist(err) {
		return
	}
	caCert = filepath.Join(parentDir, CACert)
	if _, err := os.Stat(caCert); os.IsNotExist(err) {
		return
	}
	selfSignedCert = filepath.Join(parentDir, SelfSignedCert)
	if _, err := os.Stat(selfSignedCert); os.IsNotExist(err) {
		return
	}
	selfSignedKey = filepath.Join(parentDir, SelfSignedKey)
	if _, err := os.Stat(selfSignedKey); os.IsNotExist(err) {
		return
	}
	ok = true
	return
}
