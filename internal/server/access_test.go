package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"prismflow/internal/model"
	"strings"
	"testing"
)

func TestAccessSettingsPreserveExistingEndpointsAndPartialSettings(t *testing.T) {
	s := testServer(t)
	cookie, csrf := setupLogin(t, s)
	h := s.AdminHandler()
	w := call(t, h, "POST", "/api/endpoints/preset", map[string]any{"variants": []string{"reality"}, "host": "old.example.com", "realityTarget": "www.microsoft.com:443"}, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	original := s.State().Endpoints[0]
	w = call(t, h, "PUT", "/api/settings", map[string]any{"publicUrl": "https://sub.example.com", "defaultHost": "node.example.com", "access": model.AccessSettings{BaseDomain: "example.com", PanelURL: "https://panel.example.com", XHTTPTLS: "external"}}, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if s.State().Endpoints[0] != original {
		t.Fatal("defaults changed existing endpoint")
	}
	w = call(t, h, "PUT", "/api/settings", map[string]any{"retentionDays": 42}, cookie, csrf)
	if w.Code != 200 || s.State().PublicURL != "https://sub.example.com" || s.State().Access.PanelURL != "https://panel.example.com" {
		t.Fatal(w.Body.String())
	}
	w = call(t, h, "POST", "/api/endpoints/preset", map[string]any{"variants": []string{"xhttp"}, "tlsMode": "external", "publicPort": 443}, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	e := s.State().Endpoints[1]
	if e.Host != "node.example.com" || !e.ExternalTLS() || e.CertificateID != "" || e.ClientPort() != 443 {
		t.Fatal(e)
	}
	w = call(t, h, "GET", "/api/access/config", nil, cookie, csrf)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "proxy_request_buffering off") || strings.Contains(w.Body.String(), "private-key") {
		t.Fatal(w.Body.String())
	}
	r := httptest.NewRequest("POST", "http://localhost/api/password", nil)
	r.Header.Set("Origin", "https://panel.example.com")
	if !s.sameOrigin(r) {
		t.Fatal("saved panel origin rejected")
	}
	r.Header.Set("Origin", "https://evil.example.com")
	if s.sameOrigin(r) {
		t.Fatal("foreign origin accepted")
	}
	s.Options.Origin = "https://override.example.com"
	r.Header.Set("Origin", "https://panel.example.com")
	if s.sameOrigin(r) {
		t.Fatal("env origin did not take precedence")
	}
	if w = call(t, h, "GET", "/api/access/config", nil, nil, ""); w.Code != 401 {
		t.Fatal("configuration exposed anonymously")
	}
}
func TestDeploymentChecksVerifyRoleAndDoNotFollowRedirects(t *testing.T) {
	public := httptest.NewServer(accessMarker("public"))
	defer public.Close()
	if result := checkAccessURL(context.Background(), "public", public.URL); !result.OK {
		t.Fatal(result)
	}
	if result := checkAccessURL(context.Background(), "panel", public.URL); result.OK {
		t.Fatal("wrong upstream accepted")
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, public.URL, 302) }))
	defer redirect.Close()
	if result := checkAccessURL(context.Background(), "public", redirect.URL); result.OK {
		t.Fatal("redirect accepted")
	}
}
func TestWildcardHTTPChallengeRejectedBeforeIssuance(t *testing.T) {
	s := testServer(t)
	cookie, csrf := setupLogin(t, s)
	w := call(t, s.AdminHandler(), "POST", "/api/certificates/issue", map[string]any{"domain": "*.example.com", "email": "admin@example.com", "agree": true, "method": "http"}, cookie, csrf)
	if w.Code != 400 {
		t.Fatal("wildcard HTTP-01 accepted")
	}
}
func TestSavedHTTPSOriginDefaultPortAndSecureCookie(t *testing.T) {
	s := testServer(t)
	if err := s.mutate(func(st *model.State) error { st.Access.PanelURL = "https://PANEL.example.com:443"; return nil }); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://panel.example.com/api/session", nil)
	r.Header.Set("Origin", "https://panel.example.com")
	if !s.sameOrigin(r) || !s.secureCookie(r) {
		t.Fatal("canonical HTTPS origin or secure cookie rejected")
	}
	for _, origin := range []string{"http://panel.example.com", "https://panel.example.com:444", "https://user@panel.example.com", "https://panel.example.com/path"} {
		r.Header.Set("Origin", origin)
		if s.sameOrigin(r) {
			t.Fatal("foreign origin accepted", origin)
		}
	}
}
func TestSavedPanelAddressPreservesLocalSSHTunnelSetup(t *testing.T) {
	s := testServer(t)
	if err := s.mutate(func(st *model.State) error { st.Access.PanelURL = "https://panel.example.com"; return nil }); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://localhost:8080/api/settings", nil)
	r.RemoteAddr = "127.0.0.1:54321"
	r.Header.Set("Origin", "http://localhost:8080")
	if !s.sameOrigin(r) {
		t.Fatal("local SSH setup locked out after saving public URL")
	}
	r.RemoteAddr = "192.0.2.1:1234"
	if s.sameOrigin(r) {
		t.Fatal("nonlocal setup fallback accepted")
	}
	r.RemoteAddr = "127.0.0.1:54321"
	s.Options.Origin = "https://fixed.example.com"
	if s.sameOrigin(r) {
		t.Fatal("explicit origin policy weakened")
	}
}
