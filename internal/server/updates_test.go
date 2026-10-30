package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (f updateTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPanelUpdateIsolationAndCache(t *testing.T) {
	s := testServer(t)
	s.Options.Version = "1.0.0"
	s.Options.InstallMode = "docker"
	cookie, csrf := setupLogin(t, s)
	h := s.AdminHandler()
	calls := 0
	s.panelUpdates.client = &http.Client{Transport: updateTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v1.1.0","draft":false,"prerelease":false}`)), Header: make(http.Header)}, nil
	})}
	for i := 0; i < 2; i++ {
		w := call(t, h, "POST", "/api/updates/check", nil, cookie, csrf)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "v1.1.0") {
			t.Fatal(w.Body.String())
		}
	}
	if calls != 1 {
		t.Fatal("version check cache ineffective")
	}
	for _, path := range []string{"/api/updates/install", "/api/updates/automatic"} {
		method := "POST"
		var body any
		if strings.HasSuffix(path, "automatic") {
			method = "PUT"
			body = map[string]bool{"enabled": true}
		}
		w := call(t, h, method, path, body, cookie, csrf)
		if w.Code != 400 {
			t.Fatal("non-native deployment can update", w.Code)
		}
	}
	if w := call(t, h, "POST", "/api/updates/check", nil, nil, ""); w.Code != 401 {
		t.Fatal("unauthenticated update access")
	}
	if w := call(t, h, "POST", "/api/updates/install", nil, cookie, ""); w.Code != 403 {
		t.Fatal("update missing CSRF protection")
	}
}
