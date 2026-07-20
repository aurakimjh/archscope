// CA + on-the-fly leaf minting for the T-574 build-vs-integrate spike.
//
// This is the "option 3" (own H1 semantic MVP) evidence: it shows that a
// self-contained Go binary — no external proxy library, no bundled runtime —
// can mint a per-host leaf certificate signed by a machine-local CA at
// connect time. Certificate handling is exactly where the design's §11.2 CA
// lifecycle lives, so proving it in pure stdlib de-risks the recommendation.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"
)

// mitmCA holds the local signing authority and a per-host leaf cache.
type mitmCA struct {
	caCert  *x509.Certificate
	caKey   *ecdsa.PrivateKey
	caDER   []byte
	leafKey *ecdsa.PrivateKey // one key reused across leaves (spike simplification)

	mu    sync.Mutex
	cache map[string]*tls.Certificate
}

// newMitmCA generates a fresh machine-local CA. In the product this maps to the
// §11.2 CA lifecycle (generated → trusted → expired), machine-unique, 1-year,
// never exported. Here it is in-memory and ephemeral.
func newMitmCA(notBefore time.Time) (*mitmCA, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ArchScope T-574 spike CA", Organization: []string{"ArchScope"}},
		NotBefore:             notBefore.Add(-1 * time.Hour),
		NotAfter:              notBefore.AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &mitmCA{
		caCert: caCert, caKey: caKey, caDER: der, leafKey: leafKey,
		cache: map[string]*tls.Certificate{},
	}, nil
}

// leafFor returns (minting if needed) a leaf certificate for host, signed by
// the CA. This is the GetCertificate hook used during the intercepted TLS
// handshake, so the mint cost is on the connection's critical path — a point
// the risk register calls out.
func (c *mitmCA) leafFor(host string, now time.Time) (*tls.Certificate, error) {
	c.mu.Lock()
	if tc, ok := c.cache[host]; ok {
		c.mu.Unlock()
		return tc, nil
	}
	c.mu.Unlock()

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    now.Add(-1 * time.Hour),
		NotAfter:     now.AddDate(0, 0, 397), // browser max leaf lifetime bound
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	// An IP host needs an IP SAN, not a DNS SAN, or verification fails.
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.caCert, &c.leafKey.PublicKey, c.caKey)
	if err != nil {
		return nil, fmt.Errorf("mint leaf for %s: %w", host, err)
	}
	tc := &tls.Certificate{
		Certificate: [][]byte{der, c.caDER},
		PrivateKey:  c.leafKey,
		Leaf:        nil,
	}
	c.mu.Lock()
	c.cache[host] = tc
	c.mu.Unlock()
	return tc, nil
}

// pool returns a cert pool trusting this CA (used by the self-test client).
func (c *mitmCA) pool() *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(c.caCert)
	return p
}
