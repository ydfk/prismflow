package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http/httptest"
	"net/url"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"strings"
	"testing"
	"time"
)

func TestBulkPresetsMixedTLSAndNonstandardHTTPSPort(t *testing.T) {
	s := testServer(t)
	cookie, csrf := setupLogin(t, s)
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(11), DNSNames: []string{"example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert := model.Certificate{ID: "shared-cert", Domain: "example.com", PEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), Key: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})), ExpiresAt: template.NotAfter, Method: "import"}
	user := model.NewUser("device")
	if err = s.mutate(func(st *model.State) error {
		st.PublicURL = "https://example.com:8443"
		st.Access.PanelURL = st.PublicURL
		st.DefaultHost = "example.com"
		st.Certificates = []model.Certificate{cert}
		st.DefaultCertificateID = cert.ID
		st.Users = []model.User{user}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"variants": []string{"reality", "xhttp", "anytls", "hysteria2"}, "ports": map[string]int{"reality": 27001, "xhttp": 27002, "anytls": 27003, "hysteria2": 27004}, "tlsMode": "external", "realityTarget": "www.example.net:443", "userIds": []string{user.ID}}
	response := call(t, s.AdminHandler(), "POST", "/api/endpoints/preset", body, cookie, csrf)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	st := s.State()
	if len(st.Endpoints) != 4 || len(st.Users[0].EndpointIDs) != 4 {
		t.Fatal("incomplete batch")
	}
	for _, e := range st.Endpoints {
		if e.ExternalTLS() {
			if e.PublicPort != 8443 || e.CertificateID != "" {
				t.Fatal(e)
			}
			share, err := url.Parse(core.Share(e, st.Users[0]))
			if err != nil || share.Port() != "8443" {
				t.Fatal("share port lost", share, err)
			}
		} else if e.Security == "tls" && e.CertificateID != cert.ID {
			t.Fatal("shared certificate lost for direct TLS", e)
		}
	}
	config := buildAccessConfig(st, s.Options)
	if !strings.Contains(config.Caddy, "https://example.com:8443 {") || !strings.Contains(config.Nginx, "listen 8443 ssl http2;") {
		t.Fatal("proxy port lost", config)
	}
	request := httptest.NewRequest("POST", "http://example.com:8443/manage/api/state", nil)
	request.Header.Set("Origin", "https://example.com:8443")
	if !s.sameOrigin(request) || !strings.Contains(config.Environment, "PRISMFLOW_ORIGIN=https://example.com:8443") {
		t.Fatal("custom HTTPS origin lost")
	}
	request.Header.Set("Origin", "https://example.com")
	if s.sameOrigin(request) {
		t.Fatal("default-port origin incorrectly accepted")
	}
	if !strings.Contains(core.Links(st, st.Users[0]), ":8443") {
		t.Fatal("subscription port lost")
	}
	revision := st.Revision
	body["variants"] = []string{"reality", "xhttp"}
	body["ports"] = map[string]int{"reality": 28001, "xhttp": 27002}
	failed := call(t, s.AdminHandler(), "POST", "/api/endpoints/preset", body, cookie, csrf)
	if failed.Code != 400 || s.State().Revision != revision || len(s.State().Endpoints) != 4 || len(s.State().Users[0].EndpointIDs) != 4 {
		t.Fatal("failed batch partially committed")
	}
	migrated := call(t, s.AdminHandler(), "PUT", "/api/settings", map[string]any{"publicUrl": "https://example.com:10443", "defaultHost": "example.com", "applyHost": true, "access": model.AccessSettings{PanelURL: "https://example.com:10443"}}, cookie, csrf)
	if migrated.Code != 200 {
		t.Fatal(migrated.Body.String())
	}
	for _, endpoint := range s.State().Endpoints {
		if endpoint.ExternalTLS() && endpoint.PublicPort != 10443 {
			t.Fatal("explicit sync did not migrate XHTTP HTTPS port")
		}
		if endpoint.Port < 27001 || endpoint.Port > 27004 {
			t.Fatal("direct listener port unexpectedly changed", endpoint.Port)
		}
	}
}
func TestNonstandardHTTPSPortDefaults(t *testing.T) {
	for address, want := range map[string]int{"": 443, "https://example.com": 443, "https://example.com:8443": 8443, "https://example.com:10443": 10443, "https://[::1]:8443": 8443} {
		if got := model.WebTLSPort(address); got != want {
			t.Fatal(address, got, want)
		}
		if address != "" && !core.ValidPublicURL(address) {
			t.Fatal("custom HTTPS address rejected", address)
		}
	}
	st := model.Default()
	st.PublicURL = "https://example.com:10443"
	endpoint := model.Endpoint{Host: "example.com", TLSMode: "external", Protocol: "vless", Transport: "xhttp", Security: "tls", Port: 29001}
	if err := newEndpointDefaults(&endpoint, st, nil); err != nil || endpoint.PublicPort != 10443 {
		t.Fatal("ordinary create port lost", endpoint, err)
	}
	endpoint.PublicPort = 9443
	if err := newEndpointDefaults(&endpoint, st, nil); err != nil || endpoint.PublicPort != 9443 {
		t.Fatal("explicit port overridden", endpoint, err)
	}
}
