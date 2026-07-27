package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"sync"
	"time"
)

// Authority is a per-process, non-exportable MITM authority. Its private keys
// are generated and retained in memory only; CertificateDER is the sole
// material exposed to trust-store installation.
type Authority struct {
	mu      sync.Mutex
	cert    *x509.Certificate
	key     *ecdsa.PrivateKey
	der     []byte
	leafKey *ecdsa.PrivateKey
	cache   map[string]*tls.Certificate
	closed  bool
}

func NewAuthority(now time.Time) (*Authority, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "ArchScope Local Capture CA", Organization: []string{"ArchScope"}},
		NotBefore:    now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true, IsCA: true, MaxPathLenZero: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Authority{cert: cert, key: key, der: der, leafKey: leafKey, cache: map[string]*tls.Certificate{}}, nil
}

func (a *Authority) CertificateDER() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]byte(nil), a.der...)
}

func (a *Authority) CertificatePEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: a.CertificateDER()})
}

func (a *Authority) Pool() *x509.CertPool {
	pool := x509.NewCertPool()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cert != nil {
		pool.AddCert(a.cert)
	}
	return pool
}

func (a *Authority) LeafFor(host string, now time.Time) (*tls.Certificate, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.cert == nil || a.key == nil || a.leafKey == nil {
		return nil, errors.New("capture authority is closed")
	}
	if cert := a.cache[host]; cert != nil {
		return cert, nil
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	notAfter := now.AddDate(0, 0, 397)
	if a.cert.NotAfter.Before(notAfter) {
		notAfter = a.cert.NotAfter
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: host},
		NotBefore: now.Add(-time.Hour), NotAfter: notAfter,
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, a.cert, &a.leafKey.PublicKey, a.key)
	if err != nil {
		return nil, err
	}
	cert := &tls.Certificate{Certificate: [][]byte{der, a.der}, PrivateKey: a.leafKey}
	a.cache[host] = cert
	return cert, nil
}

func (a *Authority) ExpiresAt() time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cert == nil {
		return time.Time{}
	}
	return a.cert.NotAfter
}

func (a *Authority) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.key = nil
	a.leafKey = nil
	a.cert = nil
	for key := range a.der {
		a.der[key] = 0
	}
	a.der = nil
	a.cache = map[string]*tls.Certificate{}
	a.closed = true
}
