package cert

import (
	"crypto/x509"
	"unsafe"

	"github.com/luskaner/ageServer/common"
	"golang.org/x/sys/windows"
)

const rootStoreName = "ROOT"

// Injectable wrappers for Windows API – allows unit tests to mock the store
// without touching the real certificate store.
var (
	certOpenStoreFn                    = windows.CertOpenStore
	certCloseStoreFn                   = windows.CertCloseStore
	certCreateCertificateContextFn     = windows.CertCreateCertificateContext
	certAddCertificateContextToStoreFn = windows.CertAddCertificateContextToStore
	certFindCertificateInStoreFn       = windows.CertFindCertificateInStore
	certDeleteCertificateFromStoreFn   = windows.CertDeleteCertificateFromStore
	certEnumCertificatesInStoreFn      = windows.CertEnumCertificatesInStore
	certFreeCertificateContextFn       = windows.CertFreeCertificateContext
)

func openNamedStore(userStore bool, storeName string) (windows.Handle, error) {
	rootStr := windows.StringToUTF16Ptr(storeName)
	var flags uint32
	if userStore {
		flags = windows.CERT_SYSTEM_STORE_CURRENT_USER
	} else {
		flags = windows.CERT_SYSTEM_STORE_LOCAL_MACHINE
	}
	return certOpenStoreFn(windows.CERT_STORE_PROV_SYSTEM, 0, 0, flags, uintptr(unsafe.Pointer(rootStr)))
}

func TrustCertificates(userStore bool, certs []*x509.Certificate) error {
	return trustCertificatesInStore(userStore, rootStoreName, certs)
}

func trustCertificatesInStore(userStore bool, storeName string, certs []*x509.Certificate) error {
	store, err := openNamedStore(userStore, storeName)
	if err != nil {
		return err
	}
	defer func(store windows.Handle, flags uint32) {
		_ = certCloseStoreFn(store, flags)
	}(store, 0)

	for _, cert := range certs {
		certBytes := cert.Raw
		var certContext *windows.CertContext
		certContext, err = certCreateCertificateContextFn(windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING, &certBytes[0], uint32(len(certBytes)))
		if err != nil {
			return err
		}
		err = certAddCertificateContextToStoreFn(store, certContext, windows.CERT_STORE_ADD_NEW, nil)
		_ = certFreeCertificateContextFn(certContext)
		if err != nil {
			return err
		}
	}
	return nil
}

func UntrustCertificates(userStore bool) (certs []*x509.Certificate, err error) {
	return untrustCertificatesFromStore(userStore, rootStoreName)
}

func untrustCertificatesFromStore(userStore bool, storeName string) (certs []*x509.Certificate, err error) {
	return iterateContext(
		userStore,
		storeName,
		func(store windows.Handle, _ *windows.CertContext) (*windows.CertContext, error) {
			return certFindCertificateInStoreFn(store, windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING, 0, windows.CERT_FIND_SUBJECT_STR, unsafe.Pointer(windows.StringToUTF16Ptr(common.CertSubjectOrganization)), nil)
		},
		func(certContext *windows.CertContext) error {
			// CertDeleteCertificateFromStore always frees the context itself,
			// even on failure. Freeing it again here would be a double free.
			return certDeleteCertificateFromStoreFn(certContext)
		},
		true,
	)
}

// iterateContext walks certificate contexts obtained from getter and applies
// action to each. When actionTakesOwnership is true, the action (or the API it
// calls) consumes each context; otherwise the final context is freed here.
func iterateContext(userStore bool, storeName string, contextGetter func(store windows.Handle, prevCertContext *windows.CertContext) (*windows.CertContext, error), action func(*windows.CertContext) error, actionTakesOwnership bool) (certs []*x509.Certificate, err error) {
	var store windows.Handle
	store, err = openNamedStore(userStore, storeName)
	if err != nil {
		return
	}
	defer func(store windows.Handle, flags uint32) {
		_ = certCloseStoreFn(store, flags)
	}(store, 0)
	certs = make([]*x509.Certificate, 0)
	var certContext *windows.CertContext
	for {
		certContext, err = contextGetter(store, certContext)
		if certContext == nil || err != nil {
			if len(certs) > 0 {
				err = nil
			}
			break
		}
		certBytes := make([]byte, certContext.Length)
		for i := range certBytes {
			certBytes[i] = *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(certContext.EncodedCert)) + uintptr(i)))
		}
		var cert *x509.Certificate
		cert, err = x509.ParseCertificate(certBytes)
		if err != nil {
			break
		}
		certs = append(certs, cert)

		err = action(certContext)
		if actionTakesOwnership {
			certContext = nil
		}
		if err != nil {
			break
		}
	}
	if certContext != nil {
		_ = certFreeCertificateContextFn(certContext)
	}
	return
}

func EnumCertificates(userStore bool) (certs []*x509.Certificate, err error) {
	return enumCertificatesInStore(userStore, rootStoreName)
}

func enumCertificatesInStore(userStore bool, storeName string) (certs []*x509.Certificate, err error) {
	return iterateContext(
		userStore,
		storeName,
		certEnumCertificatesInStoreFn,
		func(*windows.CertContext) error {
			return nil
		},
		false,
	)
}
