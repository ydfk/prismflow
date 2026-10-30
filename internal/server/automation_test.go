package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"strings"
	"testing"
	"time"
)

func TestSystemCertificatePathAndBrowser(t *testing.T) {
	s := testServer(t)
	cookie, csrf := setupLogin(t, s)
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), DNSNames: []string{"node.example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	certPath := filepath.Join(dir, "fullchain.pem")
	keyPath := filepath.Join(dir, "privkey.pem")
	if err = os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
	body := map[string]string{"domain": "node.example.com", "certPath": certPath, "keyPath": keyPath}
	w := call(t, s.AdminHandler(), "POST", "/api/certificates/path", body, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	listing := call(t, s.AdminHandler(), "GET", "/api/certificates/files?path="+url.QueryEscape(dir), nil, cookie, csrf)
	if listing.Code != 200 || !strings.Contains(listing.Body.String(), "fullchain.pem") || strings.Contains(listing.Body.String(), "PRIVATE KEY") {
		t.Fatal("certificate browser did not return safe file metadata")
	}
	preset := map[string]any{"variants": []string{"reality", "xhttp", "anytls", "hysteria2"}, "host": "127.0.0.1", "ports": map[string]int{"reality": 24443, "xhttp": 24444, "anytls": 24445, "hysteria2": 24446}, "certificateId": s.State().Certificates[0].ID, "realityTarget": "www.example.com:443"}
	w = call(t, s.AdminHandler(), "POST", "/api/endpoints/preset", preset, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	entries := s.State().Endpoints
	if len(entries) != 4 || entries[0].ServerName != "www.example.com" || entries[1].ServerName != "node.example.com" || entries[2].Port != 24445 {
		t.Fatal("recommended preset did not keep REALITY and TLS names separate")
	}
	c := s.State().Certificates[0]
	if c.Method != "path" || c.CertPath != certPath || c.PEM == "" {
		t.Fatal("system certificate not linked")
	}
	if err = os.WriteFile(certPath, []byte("invalid replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.mutate(func(st *model.State) error { return nil }); err == nil {
		t.Fatal("invalid replacement accepted")
	}
	if s.State().Certificates[0].PEM != c.PEM {
		t.Fatal("invalid replacement changed saved certificate")
	}
	s.refreshSystemCertificateFiles()
	if s.State().Certificates[0].LastError == "" || s.State().Certificates[0].PEM != c.PEM {
		t.Fatal("failed file refresh did not preserve certificate and report status")
	}
	if err = os.WriteFile(certPath, []byte(c.PEM), 0600); err != nil {
		t.Fatal(err)
	}
	s.refreshSystemCertificateFiles()
	if s.State().Certificates[0].LastError != "" {
		t.Fatal("recovered certificate did not clear error")
	}
	w = call(t, s.AdminHandler(), "GET", "/api/state", nil, cookie, csrf)
	if json.Valid(w.Body.Bytes()) == false || strings.Contains(w.Body.String(), "PRIVATE KEY") {
		t.Fatal("state leaked certificate private key")
	}
}
func TestPresetAtomicityAndUserSelection(t *testing.T) {
	s := testServer(t)
	cookie, csrf := setupLogin(t, s)
	u := model.NewUser("phone")
	s.state.Users = append(s.state.Users, u)
	body := map[string]any{"variants": []string{"reality"}, "host": "127.0.0.1", "ports": map[string]int{"reality": 23458}, "realityTarget": "www.example.com:443", "userIds": []string{u.ID}}
	w := call(t, s.AdminHandler(), "POST", "/api/endpoints/preset", body, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	st := s.State()
	if len(st.Endpoints) != 1 || len(st.Users[0].EndpointIDs) != 1 {
		t.Fatal("preset assignment missing")
	}
	revision := st.Revision
	w = call(t, s.AdminHandler(), "POST", "/api/endpoints/preset", body, cookie, csrf)
	if w.Code != 400 || s.State().Revision != revision || len(s.State().Endpoints) != 1 {
		t.Fatal("conflicting preset partially saved")
	}
}
func TestIntegrationAutomaticUserApply(t *testing.T) {
	binary := os.Getenv("MIHOMO_BINARY")
	if binary == "" {
		t.Skip("requires MIHOMO_BINARY")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	s := testServer(t)
	s.Core.SetBinary(binary)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	controller := l.Addr().String()
	l.Close()
	s.Options.Controller = controller
	s.Core = core.NewManager(binary, filepath.Join(s.Options.DataDir, "core-test"), controller, model.Token(24))
	e := model.Endpoint{Name: "auto", Protocol: "shadowsocks", Host: "127.0.0.1", Port: 23538, Transport: "tcp", Security: "none", Enabled: true}
	if err = model.InitEndpoint(&e); err != nil {
		t.Fatal(err)
	}
	if err = s.mutate(func(st *model.State) error { st.Endpoints = append(st.Endpoints, e); return nil }); err != nil {
		t.Fatal(err)
	}
	u := model.NewUser("phone")
	u.EndpointIDs = []string{e.ID}
	if err = s.mutate(func(st *model.State) error { st.Users = append(st.Users, u); return nil }); err != nil {
		t.Fatal(err)
	}
	if !s.Core.Running() || s.Applied().Revision != s.State().Revision {
		t.Fatal("saving user did not apply automatically")
	}
	if err = s.mutate(func(st *model.State) error { st.Users[0].EndpointIDs = nil; return nil }); err != nil {
		t.Fatal(err)
	}
	if len(s.Applied().Users[0].EndpointIDs) != 0 {
		t.Fatal("user protocol deselection not applied")
	}
	var version map[string]any
	if err = s.Core.Request(context.Background(), "/version", &version); err != nil {
		t.Fatal(err)
	}
}
