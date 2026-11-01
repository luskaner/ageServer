package wrapper

import "crypto/x509"

func RemoveUserCerts() (crts []*x509.Certificate, err error) {
	// Must not be called
	return nil, nil
}

// AddUserCerts takes the same signature as everywhere else even though nothing
// happens here. With an `any` parameter instead, every caller and every test
// that assigned the []*x509.Certificate form failed to compile on Linux.
func AddUserCerts(_ []*x509.Certificate) error {
	// Must not be called
	return nil
}
