package authn

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DefaultCertValidity is the validity duration for ephemeral mTLS certificates.
const DefaultCertValidity = 24 * time.Hour

// Bundle holds PEM-encoded mTLS certificate material.
// A Bundle is safe for concurrent use once created; its fields should not be modified.
type Bundle struct {
	CACertPEM     []byte
	ServerCertPEM []byte
	ServerKeyPEM  []byte
	ClientCertPEM []byte
	ClientKeyPEM  []byte
}

// GenerateBundle creates a complete set of ephemeral certificates for mTLS authentication.
// It generates a self-signed CA, a server certificate, and a client certificate.
// All certificates are valid for 24 hours and use ECDSA P-256 keys.
// extraSANs are additional DNS names or IP addresses to add to the server certificate.
func GenerateBundle(extraSANs ...string) (*Bundle, error) {
	// Generate CA
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	caSerial, err := randomSerial()
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA serial: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(DefaultCertValidity)

	// Note: CA is constrained by MaxPathLen=0 (cannot issue sub-CAs).
	// ExtKeyUsage is intentionally omitted as it's not meaningful for CA certs
	// and Go's x509 package doesn't enforce it on CA certificates.
	caTemplate := &x509.Certificate{
		SerialNumber: caSerial,
		Subject: pkix.Name{
			CommonName: "azd-auth-ca",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertDER,
	})

	// Generate Server Certificate
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server key: %w", err)
	}

	serverSerial, err := randomSerial()
	if err != nil {
		return nil, fmt.Errorf("failed to generate server serial: %w", err)
	}

	dnsNames := []string{
		"localhost",
		"host.docker.internal",
		"host.containers.internal",
		"host.lima.internal",
		"host.rancher-desktop.internal",
	}
	ipAddresses := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("::1"),
	}

	// Add extra SANs (IPs or hostnames)
	for _, san := range extraSANs {
		san = strings.TrimSpace(san)
		if san == "" || len(san) > 253 {
			continue
		}
		if ip := net.ParseIP(san); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else {
			dnsNames = append(dnsNames, san)
		}
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject: pkix.Name{
			CommonName: "azd-auth-server",
		},
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    dnsNames,
		IPAddresses: ipAddresses,
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create server certificate: %w", err)
	}

	serverCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertDER,
	})

	serverKeyDER, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal server key: %w", err)
	}

	serverKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: serverKeyDER,
	})
	serverKey.D.SetInt64(0) // Zero private key material
	runtime.KeepAlive(serverKey)

	// Generate Client Certificate
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client key: %w", err)
	}

	clientSerial, err := randomSerial()
	if err != nil {
		return nil, fmt.Errorf("failed to generate client serial: %w", err)
	}

	clientTemplate := &x509.Certificate{
		SerialNumber: clientSerial,
		Subject: pkix.Name{
			CommonName: "azd-auth-client",
		},
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create client certificate: %w", err)
	}

	clientCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCertDER,
	})

	clientKeyDER, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal client key: %w", err)
	}

	clientKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: clientKeyDER,
	})
	clientKey.D.SetInt64(0) // Zero private key material
	caKey.D.SetInt64(0)     // Zero private key material
	runtime.KeepAlive(clientKey)
	runtime.KeepAlive(caKey)

	return &Bundle{
		CACertPEM:     caCertPEM,
		ServerCertPEM: serverCertPEM,
		ServerKeyPEM:  serverKeyPEM,
		ClientCertPEM: clientCertPEM,
		ClientKeyPEM:  clientKeyPEM,
	}, nil
}

// ServerTLSConfig creates a tls.Config for the server with mTLS enabled.
// It requires and verifies client certificates using the CA from the bundle.
func ServerTLSConfig(b *Bundle) (*tls.Config, error) {
	serverCert, err := tls.X509KeyPair(b.ServerCertPEM, b.ServerKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(b.CACertPEM) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// ClientTLSConfig creates a tls.Config for the client with mTLS enabled.
// It uses the client certificate and trusts the CA from the bundle.
func ClientTLSConfig(b *Bundle) (*tls.Config, error) {
	clientCert, err := tls.X509KeyPair(b.ClientCertPEM, b.ClientKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(b.CACertPEM) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// WriteCertsToDir writes the CA certificate, client certificate, and client key
// to the specified directory. These files can be mounted into a container.
// The files created are:
//   - ca.pem: CA certificate
//   - client.pem: Client certificate
//   - client-key.pem: Client private key
func WriteCertsToDir(b *Bundle, dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Verify directory is not a symlink (TOCTOU mitigation)
	fi, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("failed to stat certs directory: %w", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("certs directory is a symlink, refusing to write (possible attack)")
	}

	files := map[string][]byte{
		"ca.pem":         b.CACertPEM,
		"client.pem":     b.ClientCertPEM,
		"client-key.pem": b.ClientKeyPEM,
	}

	for filename, content := range files {
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, content, 0600); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}

// Validate checks that all required PEM fields in the bundle are non-empty.
func (b *Bundle) Validate() error {
	if len(b.CACertPEM) == 0 {
		return fmt.Errorf("bundle missing CA certificate")
	}
	if len(b.ServerCertPEM) == 0 || len(b.ServerKeyPEM) == 0 {
		return fmt.Errorf("bundle missing server certificate or key")
	}
	if len(b.ClientCertPEM) == 0 || len(b.ClientKeyPEM) == 0 {
		return fmt.Errorf("bundle missing client certificate or key")
	}
	return nil
}

// randomSerial generates a random serial number for X.509 certificates.
func randomSerial() (*big.Int, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 160)
	serial, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}
	return serial, nil
}
