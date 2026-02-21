// Package tlsroots builds a merged TLS root CA pool that combines the
// system certificate pool with the embedded Mozilla NSS fallback bundle.
//
// This handles the case where the system cert bundle is present but incomplete
// (e.g. missing a specific root CA), which would cause x509 verification errors
// for some sites even though Go's fallback mechanism is not triggered (it only
// activates when there is no system pool at all).
package tlsroots

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"

	"golang.org/x/crypto/x509roots/fallback/bundle"
)

// NewHTTPClient returns an *http.Client whose TLS configuration uses a merged
// root pool: the system pool (if available) augmented with every root from the
// embedded Mozilla NSS bundle. Certificates already in the system pool are not
// duplicated.
//
// timeout and checkRedirect are passed through unchanged.
func NewTransport() *http.Transport {
	pool := mergedPool()
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}
}

// mergedPool returns a *x509.CertPool that contains the union of the system
// roots and the embedded Mozilla bundle roots.
func mergedPool() *x509.CertPool {
	// Start from the system pool. If unavailable (e.g. scratch container),
	// start empty — the fallback bundle will cover it.
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}

	// Add every root from the embedded Mozilla NSS bundle. x509.CertPool
	// deduplicates by subject+SPKI, so re-adding an already-present cert is
	// harmless.
	for root := range bundle.Roots() {
		if root.Constraint == nil {
			pool.AddCert(mustParse(root.Certificate))
		} else {
			pool.AddCertWithConstraint(mustParse(root.Certificate), root.Constraint)
		}
	}

	return pool
}

// mustParse parses a DER-encoded certificate and panics on failure. The
// embedded bundle certs are known-good, so this should never fail in practice.
func mustParse(der []byte) *x509.Certificate {
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		panic("tlsroots: failed to parse embedded certificate: " + err.Error())
	}
	return cert
}
