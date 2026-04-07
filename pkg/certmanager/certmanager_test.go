package certmanager

import (
	"crypto/ed25519"
	"crypto/x509"
	"testing"
)

func TestGenerateCA(t *testing.T) {
	ca, err := GenerateCA()
	if err != nil {
		t.Fatal(err)
	}
	if !ca.Cert.IsCA {
		t.Error("cert is not CA")
	}
	if ca.Cert.Subject.CommonName != "westcoast-cluster-ca" {
		t.Errorf("CN = %q", ca.Cert.Subject.CommonName)
	}
	if len(ca.Key) != ed25519.PrivateKeySize {
		t.Error("key is not ed25519")
	}
}

func TestGenerateCSR(t *testing.T) {
	csrDER, key, err := GenerateCSR("node-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != ed25519.PrivateKeySize {
		t.Error("key is not ed25519")
	}

	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		t.Fatal(err)
	}
	if csr.Subject.CommonName != "node-1" {
		t.Errorf("CN = %q", csr.Subject.CommonName)
	}
}

func TestSignCSR(t *testing.T) {
	ca, err := GenerateCA()
	if err != nil {
		t.Fatal(err)
	}

	csrDER, _, err := GenerateCSR("node-1")
	if err != nil {
		t.Fatal(err)
	}

	certDER, err := SignCSR(ca, csrDER, "10.0.0.1:9000")
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatal(err)
	}

	if cert.Subject.CommonName != "node-1" {
		t.Errorf("CN = %q", cert.Subject.CommonName)
	}

	// Verify cert is signed by CA.
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	if _, err := cert.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
		t.Errorf("cert does not verify against CA: %v", err)
	}

	// Check IP SAN.
	if len(cert.IPAddresses) != 1 || cert.IPAddresses[0].String() != "10.0.0.1" {
		t.Errorf("IP SANs = %v", cert.IPAddresses)
	}
}

func TestGenerateNodeCert(t *testing.T) {
	ca, err := GenerateCA()
	if err != nil {
		t.Fatal(err)
	}

	node, err := GenerateNodeCert(ca, "node-1", "10.0.0.1:9000")
	if err != nil {
		t.Fatal(err)
	}

	if node.Cert.Subject.CommonName != "node-1" {
		t.Errorf("CN = %q", node.Cert.Subject.CommonName)
	}

	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)
	if _, err := node.Cert.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
		t.Errorf("cert does not verify against CA: %v", err)
	}
}

func TestSaveAndLoadCerts(t *testing.T) {
	ca, err := GenerateCA()
	if err != nil {
		t.Fatal(err)
	}
	node, err := GenerateNodeCert(ca, "node-1", "127.0.0.1:9000")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := SaveCerts(dir, ca, node); err != nil {
		t.Fatal(err)
	}

	if !CertsExist(dir) {
		t.Fatal("CertsExist returned false after save")
	}

	ca2, node2, err := LoadCerts(dir)
	if err != nil {
		t.Fatal(err)
	}

	if ca2.Cert.Subject.CommonName != ca.Cert.Subject.CommonName {
		t.Error("CA cert mismatch after load")
	}
	if node2.Cert.Subject.CommonName != node.Cert.Subject.CommonName {
		t.Error("node cert mismatch after load")
	}

	// Verify loaded certs work for signing.
	csrDER, _, err := GenerateCSR("node-2")
	if err != nil {
		t.Fatal(err)
	}
	certDER, err := SignCSR(ca2, csrDER, "127.0.0.2:9000")
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(certDER)
	pool := x509.NewCertPool()
	pool.AddCert(ca2.Cert)
	if _, err := cert.Verify(x509.VerifyOptions{Roots: pool}); err != nil {
		t.Error("cert signed with loaded CA doesn't verify")
	}
}

func TestCertsExist_Empty(t *testing.T) {
	if CertsExist(t.TempDir()) {
		t.Error("CertsExist returned true for empty dir")
	}
}

func TestTLSConfigs(t *testing.T) {
	ca, err := GenerateCA()
	if err != nil {
		t.Fatal(err)
	}
	node, err := GenerateNodeCert(ca, "node-1", "127.0.0.1:9000")
	if err != nil {
		t.Fatal(err)
	}

	server := ServerTLSConfig(ca, node)
	if server.ClientAuth != 4 { // tls.RequireAndVerifyClientCert
		t.Error("server should require client certs")
	}
	if len(server.Certificates) != 1 {
		t.Error("server should have 1 certificate")
	}

	client := ClientTLSConfig(ca, node)
	if client.RootCAs == nil {
		t.Error("client should have root CAs")
	}
	if len(client.Certificates) != 1 {
		t.Error("client should have 1 certificate")
	}
}
