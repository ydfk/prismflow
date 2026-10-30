package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReuseSystemWildcardCertificateForDirectProtocols(t *testing.T) {
	s := testServer(t)
	cookie, csrf := setupLogin(t, s)
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(7), DNSNames: []string{"*.example.com", "example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, "fullchain.pem")
	keyPath := filepath.Join(dir, "privkey.pem")
	if err = os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
	h := s.AdminHandler()
	w := call(t, h, "POST", "/api/certificates/path", map[string]any{"domain": "*.example.com", "certPath": certPath, "keyPath": keyPath}, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(t, h, "POST", "/api/endpoints/preset", map[string]any{"variants": []string{"xhttp", "anytls", "hysteria2"}, "host": "node.example.com", "tlsMode": "core", "certificateId": s.State().Certificates[0].ID}, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, e := range s.State().Endpoints {
		if e.ServerName != "node.example.com" {
			t.Fatal("wildcard was used as SNI")
		}
	}
	w = call(t, h, "GET", "/api/state", nil, cookie, csrf)
	if !strings.Contains(w.Body.String(), `"names":["*.example.com","example.com"]`) || strings.Contains(w.Body.String(), "PRIVATE KEY") {
		t.Fatal("certificate metadata missing or secret leaked")
	}
}
