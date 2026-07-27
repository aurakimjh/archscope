//go:build windows

package certstore

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const certificateEncoding = windows.X509_ASN_ENCODING | windows.PKCS_7_ASN_ENCODING

type systemBackend struct{}

func (systemBackend) Install(storeName string, certificateDER []byte) error {
	store, err := openStore(storeName)
	if err != nil {
		return err
	}
	defer windows.CertCloseStore(store, 0)
	context, err := windows.CertCreateCertificateContext(certificateEncoding, &certificateDER[0], uint32(len(certificateDER)))
	if err != nil {
		return err
	}
	defer windows.CertFreeCertificateContext(context)
	return windows.CertAddCertificateContextToStore(store, context, windows.CERT_STORE_ADD_REPLACE_EXISTING, nil)
}

func (systemBackend) Remove(storeName string, certificateDER []byte) error {
	store, err := openStore(storeName)
	if err != nil {
		return err
	}
	defer windows.CertCloseStore(store, 0)
	context, err := windows.CertCreateCertificateContext(certificateEncoding, &certificateDER[0], uint32(len(certificateDER)))
	if err != nil {
		return err
	}
	defer windows.CertFreeCertificateContext(context)
	found, err := windows.CertFindCertificateInStore(
		store, certificateEncoding, 0, windows.CERT_FIND_EXISTING,
		unsafe.Pointer(context), nil,
	)
	if err != nil {
		if errors.Is(err, syscall.Errno(windows.CRYPT_E_NOT_FOUND)) {
			return nil
		}
		return err
	}
	return windows.CertDeleteCertificateFromStore(found)
}

func openStore(storeName string) (windows.Handle, error) {
	if storeName != CurrentUserRoot {
		return 0, fmt.Errorf("unsupported certificate store %q", storeName)
	}
	name, err := windows.UTF16PtrFromString("ROOT")
	if err != nil {
		return 0, err
	}
	return windows.CertOpenStore(
		windows.CERT_STORE_PROV_SYSTEM_W, 0, 0,
		windows.CERT_SYSTEM_STORE_CURRENT_USER,
		uintptr(unsafe.Pointer(name)),
	)
}
