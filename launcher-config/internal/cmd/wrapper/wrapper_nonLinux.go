//go:build !linux

package wrapper

import (
	"crypto/x509"

	"github.com/luskaner/ageServer/launcher-common/cert"
)

func RemoveUserCerts() (crts []*x509.Certificate, err error) {
	return cert.UntrustCertificates(true)
}

func AddUserCerts(crts []*x509.Certificate) error {
	return cert.TrustCertificates(true, crts)
}
