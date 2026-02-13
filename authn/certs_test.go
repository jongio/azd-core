package authn

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestGenerateBundle(t *testing.T) {
	bundle, err := GenerateBundle()
	if err != nil {
		t.Fatalf("GenerateBundle() error: %v", err)
	}

	if err := bundle.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	// Parse and verify CA cert
	caBlock, _ := pem.Decode(bundle.CACertPEM)
	if caBlock == nil {
		t.Fatal("failed to decode CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		t.Fatalf("failed to parse CA cert: %v", err)
	}
	if !caCert.IsCA {
		t.Error("CA cert IsCA should be true")
	}
	if caCert.MaxPathLen != 0 || !caCert.MaxPathLenZero {
		t.Errorf("CA MaxPathLen=%d, MaxPathLenZero=%v; want 0, true", caCert.MaxPathLen, caCert.MaxPathLenZero)
	}

	// Parse and verify server cert
	serverBlock, _ := pem.Decode(bundle.ServerCertPEM)
	if serverBlock == nil {
		t.Fatal("failed to decode server cert PEM")
	}
	serverCert, err := x509.ParseCertificate(serverBlock.Bytes)
	if err != nil {
		t.Fatalf("failed to parse server cert: %v", err)
	}
	hasServerAuth := false
	for _, usage := range serverCert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			hasServerAuth = true
		}
	}
	if !hasServerAuth {
		t.Error("server cert missing ExtKeyUsageServerAuth")
	}

	// Verify default SANs
	expectedDNS := []string{
		"localhost",
		"host.docker.internal",
		"host.containers.internal",
		"host.lima.internal",
		"host.rancher-desktop.internal",
	}
	for _, dns := range expectedDNS {
		found := false
		for _, san := range serverCert.DNSNames {
			if san == dns {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("server cert missing DNS SAN %q", dns)
		}
	}

	// Verify default IP SANs
	expectedIPs := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	for _, expectedIP := range expectedIPs {
		found := false
		for _, ip := range serverCert.IPAddresses {
			if ip.Equal(expectedIP) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("server cert missing IP SAN %s", expectedIP)
		}
	}

	// Parse and verify client cert
	clientBlock, _ := pem.Decode(bundle.ClientCertPEM)
	if clientBlock == nil {
		t.Fatal("failed to decode client cert PEM")
	}
	clientCert, err := x509.ParseCertificate(clientBlock.Bytes)
	if err != nil {
		t.Fatalf("failed to parse client cert: %v", err)
	}
	hasClientAuth := false
	for _, usage := range clientCert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
		}
	}
	if !hasClientAuth {
		t.Error("client cert missing ExtKeyUsageClientAuth")
	}
}

func TestGenerateBundleExtraSANs(t *testing.T) {
	bundle, err := GenerateBundle("extra.example.com", "10.0.0.1")
	if err != nil {
		t.Fatalf("GenerateBundle() error: %v", err)
	}

	serverBlock, _ := pem.Decode(bundle.ServerCertPEM)
	serverCert, err := x509.ParseCertificate(serverBlock.Bytes)
	if err != nil {
		t.Fatalf("failed to parse server cert: %v", err)
	}

	foundDNS := false
	for _, dns := range serverCert.DNSNames {
		if dns == "extra.example.com" {
			foundDNS = true
		}
	}
	if !foundDNS {
		t.Error("extra DNS SAN 'extra.example.com' not found in server cert")
	}

	foundIP := false
	for _, ip := range serverCert.IPAddresses {
		if ip.Equal(net.ParseIP("10.0.0.1")) {
			foundIP = true
		}
	}
	if !foundIP {
		t.Error("extra IP SAN '10.0.0.1' not found in server cert")
	}
}

func TestBundleMTLSHandshake(t *testing.T) {
	bundle, err := GenerateBundle()
	if err != nil {
		t.Fatalf("GenerateBundle() error: %v", err)
	}

	serverTLS, err := ServerTLSConfig(bundle)
	if err != nil {
		t.Fatalf("ServerTLSConfig() error: %v", err)
	}

	clientTLS, err := ClientTLSConfig(bundle)
	if err != nil {
		t.Fatalf("ClientTLSConfig() error: %v", err)
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverTLS)
	if err != nil {
		t.Fatalf("tls.Listen() error: %v", err)
	}
	defer ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: clientTLS},
	}
	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/ping", ln.Addr().(*net.TCPAddr).Port))
	if err != nil {
		t.Fatalf("mTLS request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestBundleValidate(t *testing.T) {
	tests := []struct {
		name    string
		bundle  Bundle
		wantErr bool
	}{
		{"empty bundle", Bundle{}, true},
		{"missing server cert", Bundle{
			CACertPEM: []byte("ca"), ClientCertPEM: []byte("c"), ClientKeyPEM: []byte("ck"),
		}, true},
		{"missing client cert", Bundle{
			CACertPEM: []byte("ca"), ServerCertPEM: []byte("s"), ServerKeyPEM: []byte("sk"),
		}, true},
		{"missing CA cert", Bundle{
			ServerCertPEM: []byte("s"), ServerKeyPEM: []byte("sk"),
			ClientCertPEM: []byte("c"), ClientKeyPEM: []byte("ck"),
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bundle.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	t.Run("full bundle", func(t *testing.T) {
		bundle, err := GenerateBundle()
		if err != nil {
			t.Fatalf("GenerateBundle() error: %v", err)
		}
		if err := bundle.Validate(); err != nil {
			t.Errorf("Validate() error = %v, want nil", err)
		}
	})
}

func TestWriteCertsToDir(t *testing.T) {
	bundle, err := GenerateBundle()
	if err != nil {
		t.Fatalf("GenerateBundle() error: %v", err)
	}

	dir := t.TempDir()
	certsDir := filepath.Join(dir, "certs")

	if err := WriteCertsToDir(bundle, certsDir); err != nil {
		t.Fatalf("WriteCertsToDir() error: %v", err)
	}

	// WriteCertsToDir writes 3 files: ca.pem, client.pem, client-key.pem
	expectedFiles := []string{"ca.pem", "client.pem", "client-key.pem"}
	for _, f := range expectedFiles {
		path := filepath.Join(certsDir, f)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("file %s not found: %v", f, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("file %s is empty", f)
		}
		// File permission checks only apply on Unix
		if runtime.GOOS != "windows" {
			if perm := info.Mode().Perm(); perm != 0600 {
				t.Errorf("file %s perm = %o, want 0600", f, perm)
			}
		}
	}

	// Directory permission check only applies on Unix
	if runtime.GOOS != "windows" {
		dirInfo, err := os.Stat(certsDir)
		if err != nil {
			t.Fatalf("stat dir error: %v", err)
		}
		if perm := dirInfo.Mode().Perm(); perm != 0700 {
			t.Errorf("dir perm = %o, want 0700", perm)
		}
	}
}

func TestCertificateExpiry(t *testing.T) {
	before := time.Now()
	bundle, err := GenerateBundle()
	after := time.Now()
	if err != nil {
		t.Fatalf("GenerateBundle failed: %v", err)
	}

	block, _ := pem.Decode(bundle.CACertPEM)
	if block == nil {
		t.Fatal("failed to decode CA cert PEM")
	}
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}

	if caCert.NotBefore.Before(before.Add(-1 * time.Second)) {
		t.Errorf("CA NotBefore %v is before test start %v", caCert.NotBefore, before)
	}
	if caCert.NotBefore.After(after.Add(1 * time.Second)) {
		t.Errorf("CA NotBefore %v is after test end %v", caCert.NotBefore, after)
	}

	expectedExpiry := caCert.NotBefore.Add(DefaultCertValidity)
	if !caCert.NotAfter.Equal(expectedExpiry) {
		t.Errorf("CA NotAfter = %v, want %v (NotBefore + %v)", caCert.NotAfter, expectedExpiry, DefaultCertValidity)
	}
}

func TestGenerateBundleExtraSANsEdgeCases(t *testing.T) {
	longSAN := strings.Repeat("a", 254) // >253 chars, should be ignored
	bundle, err := GenerateBundle("", "  ", longSAN, "valid.example.com")
	if err != nil {
		t.Fatalf("GenerateBundle failed: %v", err)
	}

	block, _ := pem.Decode(bundle.ServerCertPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse server cert: %v", err)
	}

	// valid.example.com should be present
	found := false
	for _, name := range cert.DNSNames {
		if name == "valid.example.com" {
			found = true
		}
		// longSAN should NOT be present
		if name == longSAN {
			t.Error("long SAN (>253 chars) should have been filtered out")
		}
	}
	if !found {
		t.Error("valid.example.com should be in server cert DNSNames")
	}
}

func TestWriteCertsToDirIdempotent(t *testing.T) {
	bundle, err := GenerateBundle()
	if err != nil {
		t.Fatalf("GenerateBundle failed: %v", err)
	}

	dir := t.TempDir()
	if err := WriteCertsToDir(bundle, dir); err != nil {
		t.Fatalf("first WriteCertsToDir failed: %v", err)
	}

	// Writing again should succeed (overwrite)
	if err := WriteCertsToDir(bundle, dir); err != nil {
		t.Fatalf("second WriteCertsToDir failed: %v", err)
	}

	// Verify files exist
	for _, f := range []string{"ca.pem", "client.pem", "client-key.pem"} {
		if _, err := os.Stat(filepath.Join(dir, f)); os.IsNotExist(err) {
			t.Errorf("expected %s to exist after second write", f)
		}
	}
}
