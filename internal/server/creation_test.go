package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"prismflow/internal/model"
	"strings"
	"testing"
	"time"
)

func TestUnifiedDomainAndRandomPorts(t *testing.T) {
	s := testServer(t)
	cookie, csrf := setupLogin(t, s)
	h := s.AdminHandler()
	w := call(t, h, "PUT", "/api/settings", map[string]any{"publicUrl": "https://sub.example.com", "retentionDays": 30, "defaultHost": "node.example.com", "applyHost": true}, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(t, h, "POST", "/api/ports/random", map[string]int{"count": 4}, cookie, csrf)
	var ports []int
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &ports) != nil || len(ports) != 4 {
		t.Fatal(w.Body.String())
	}
	seen := map[int]bool{}
	for _, p := range ports {
		if p < 10000 || p > 65535 || seen[p] {
			t.Fatal("ports not five-digit unique")
		}
		seen[p] = true
	}
	w = call(t, h, "POST", "/api/endpoints/preset", map[string]any{"variants": []string{"reality"}, "ports": map[string]int{"reality": ports[0]}, "realityTarget": "www.microsoft.com:443"}, cookie, csrf)
	if w.Code != 200 || s.State().Endpoints[0].Host != "node.example.com" {
		t.Fatal("new entry did not inherit global domain")
	}
	w = call(t, h, "PUT", "/api/settings", map[string]any{"publicUrl": "https://sub.example.com", "retentionDays": 30, "defaultHost": "new.example.com", "applyHost": true}, cookie, csrf)
	if w.Code != 200 || s.State().Endpoints[0].Host != "new.example.com" {
		t.Fatal("domain not synchronized")
	}
	old := s.State().Revision
	w = call(t, h, "POST", "/api/endpoints/preset", map[string]any{"variants": []string{"shadowsocks"}}, cookie, csrf)
	if w.Code != 400 || s.State().Revision != old {
		t.Fatal("unsupported protocol accepted")
	}
	if err := s.mutate(func(st *model.State) error { st.DefaultHost = "https://bad.example.com"; return nil }); err == nil {
		t.Fatal("invalid global host accepted")
	}
}
func TestRealityProbeTLSAndPrivateTargets(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "::1", "fd00::1"} {
		if publicTargetIP(net.ParseIP(address)) {
			t.Fatalf("private target accepted %s", address)
		}
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.EnableHTTP2 = true
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13}
	server.StartTLS()
	defer server.Close()
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result := checkTargetTLS(ctx, strings.TrimPrefix(server.URL, "https://"), "example.com", pool)
	// httptest 的内置证书覆盖 example.com；整个握手仍执行证书验证。
	if !result.Supported || !result.TLS13 || !result.H2 {
		t.Fatalf("valid TLS13 h2 server rejected: %+v", result)
	}
	blocked := probeTarget(ctx, "127.0.0.1:443")
	if blocked.Supported || blocked.Message != "仅检测公网目标" {
		t.Fatal("loopback target was probed")
	}
	s := testServer(t)
	if w := call(t, s.AdminHandler(), "POST", "/api/reality/test", map[string]any{"targets": []string{"www.microsoft.com:443"}}, nil, ""); w.Code != 401 {
		t.Fatal("anonymous target probe accepted")
	}
}
