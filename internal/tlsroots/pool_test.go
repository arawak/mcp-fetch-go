package tlsroots_test

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/arawak/mcp-fetch-go/internal/tlsroots"
)

// TestMergedPool_TransportNotNil confirms NewTransport returns a usable value.
func TestMergedPool_TransportNotNil(t *testing.T) {
	transport := tlsroots.NewTransport()
	if transport == nil {
		t.Fatal("NewTransport returned nil")
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("NewTransport returned transport with nil TLSClientConfig")
	}
	if transport.TLSClientConfig.RootCAs == nil {
		t.Fatal("NewTransport returned transport with nil RootCAs pool")
	}
}

// TestMergedPool_TLSHandshake_GoDev verifies the merged pool can complete a
// TLS handshake against go.dev, which uses Google Trust Services roots present
// in the Mozilla NSS bundle. This exercises the real merged-pool path without
// depending on the OS verifier.
//
// Skipped if the host is unreachable (offline CI).
func TestMergedPool_TLSHandshake_GoDev(t *testing.T) {
	conn, err := net.DialTimeout("tcp", "go.dev:443", 5*time.Second)
	if err != nil {
		t.Skipf("skipping: cannot reach go.dev: %v", err)
	}
	defer conn.Close()

	transport := tlsroots.NewTransport()
	pool := transport.TLSClientConfig.RootCAs

	tlsConn := tls.Client(conn, &tls.Config{
		ServerName: "go.dev",
		RootCAs:    pool,
	})
	tlsConn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := tlsConn.Handshake(); err != nil {
		t.Fatalf("TLS handshake to go.dev failed with merged pool: %v", err)
	}
	t.Log("TLS handshake to go.dev succeeded with merged pool")
}

// TestMergedPool_TLSHandshake_GitHub verifies the merged pool handles GitHub,
// which uses a DigiCert root that may also be absent from some incomplete
// system bundles.
//
// Skipped if the host is unreachable (offline CI).
func TestMergedPool_TLSHandshake_GitHub(t *testing.T) {
	conn, err := net.DialTimeout("tcp", "github.com:443", 5*time.Second)
	if err != nil {
		t.Skipf("skipping: cannot reach github.com: %v", err)
	}
	defer conn.Close()

	transport := tlsroots.NewTransport()
	pool := transport.TLSClientConfig.RootCAs

	tlsConn := tls.Client(conn, &tls.Config{
		ServerName: "github.com",
		RootCAs:    pool,
	})
	tlsConn.SetDeadline(time.Now().Add(5 * time.Second))
	if err := tlsConn.Handshake(); err != nil {
		t.Fatalf("TLS handshake to github.com failed with merged pool: %v", err)
	}
	t.Log("TLS handshake to github.com succeeded with merged pool")
}
