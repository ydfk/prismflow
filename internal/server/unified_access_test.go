package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"prismflow/internal/model"
	"strings"
	"testing"
	"time"
)

func TestUnifiedPublicEntryRoutesAndIsolation(t *testing.T) {
	s := testServer(t)
	if err := s.mutate(func(st *model.State) error { st.Access.AdminPath = "/console-test"; return nil }); err != nil {
		t.Fatal(err)
	}
	h := s.PublicHandler()
	for _, test := range []struct {
		path string
		code int
		part string
	}{
		{"/", 200, ""}, {"/api/state", 404, ""}, {"/console-test", 307, ""},
		{"/console-test/", 200, "./assets/"}, {"/console-test/api/status", 200, "setupRequired"},
		{"/console-test/api/state", 401, "请先登录"}, {"/console-test/.well-known/prismflow-access", 200, "panel"},
		{"/.well-known/prismflow-access", 200, "public"}, {"/s/invalid", 404, ""}, {"/console-test-other/api/status", 404, ""},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, test.path, nil))
		if w.Code != test.code || !strings.Contains(w.Body.String(), test.part) {
			t.Fatalf("%s: %d %s", test.path, w.Code, w.Body.String())
		}
	}
	cookie, csrf := setupLogin(t, s)
	w := call(t, h, "PUT", "/console-test/api/settings", map[string]any{"publicUrl": "https://example.com", "defaultHost": "example.com", "access": model.AccessSettings{PanelURL: "https://example.com", AdminPath: "/console-test"}}, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	config := buildAccessConfig(s.State(), s.Options)
	if strings.Count(config.Caddy, "https://example.com:443 {") != 1 || !strings.Contains(config.Caddy, "reverse_proxy 127.0.0.1:18081") {
		t.Fatal(config.Caddy)
	}
	if err := s.mutate(func(st *model.State) error { st.Website.Disabled = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if w := call(t, h, "GET", "/console-test/api/status", nil, nil, ""); w.Code != 200 {
		t.Fatal("website switch disabled panel")
	}
}
func TestCertificateOperationsPersistAndDoNotOverwriteAutomaticSource(t *testing.T) {
	s := testServer(t)
	s.recordCertificateResult("cert-one", "example.com", "续期证书", nil)
	s.recordCertificateResult("cert-two", "other.example.com", "读取证书", context.Canceled)
	if err := s.mutate(func(st *model.State) error {
		st.Certificates = append(st.Certificates, model.Certificate{ID: "cert-auto", Domain: "example.com", Method: "http", Renew: true})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(9), DNSNames: []string{"example.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cookie, csrf := setupLogin(t, s)
	response := call(t, s.AdminHandler(), "POST", "/api/certificates/import", map[string]string{"id": "cert-auto", "domain": "example.com", "pem": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), "key": string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))}, cookie, csrf)
	if response.Code != 400 || s.State().Certificates[0].Method != "http" || !s.State().Certificates[0].Renew {
		t.Fatal("automatic renewal metadata was overwritten", response.Body.String())
	}
	// 日志写入 SQLite，重新打开同一数据目录后仍存在。
	reopened, err := New(context.Background(), s.Options)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if len(reopened.jobs) != 3 {
		t.Fatal("operation logs lost on restart")
	}
	for _, job := range reopened.jobs {
		if job.CertificateID == "" || job.Operation == "" {
			t.Fatal(job)
		}
	}
}
