package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"prismflow/internal/model"
	"testing"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(context.Background(), Options{DataDir: t.TempDir(), CoreBinary: "nonexistent-mihomo", Controller: "127.0.0.1:19097", AdminListen: "127.0.0.1:18080", PublicListen: "127.0.0.1:18081", WebsiteURL: "http://127.0.0.1:18081"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}
func call(t *testing.T, h http.Handler, method, path string, body any, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, "http://localhost"+path, reader)
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	r.Header.Set("X-CSRF-Token", csrf)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func setupLogin(t *testing.T, s *Server) (*http.Cookie, string) {
	t.Helper()
	h := s.AdminHandler()
	token, err := os.ReadFile(filepath.Join(s.Options.DataDir, "setup.token"))
	if err != nil {
		t.Fatal(err)
	}
	w := call(t, h, "POST", "/api/setup", map[string]string{"username": "admin", "password": "Test-password-12345", "token": string(token)}, nil, "")
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	w = call(t, h, "POST", "/api/session", map[string]string{"username": "admin", "password": "Test-password-12345"}, nil, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var v map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	return w.Result().Cookies()[0], v["csrf"]
}
func TestAuthenticationAndCSRF(t *testing.T) {
	s := testServer(t)
	h := s.AdminHandler()
	if w := call(t, h, "GET", "/api/state", nil, nil, ""); w.Code != 401 {
		t.Fatal("anonymous state access")
	}
	cookie, csrf := setupLogin(t, s)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("cookie flags")
	}
	body := map[string]any{"publicUrl": "https://sub.example.com", "retentionDays": 30}
	if w := call(t, h, "PUT", "/api/settings", body, cookie, ""); w.Code != 403 {
		t.Fatal("CSRF accepted")
	}
	if w := call(t, h, "PUT", "/api/settings", body, cookie, csrf); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	r := httptest.NewRequest("PUT", "http://localhost/api/settings", bytes.NewBufferString(`{"publicUrl":"","retentionDays":30}`))
	r.AddCookie(cookie)
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("X-CSRF-Token", csrf)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("foreign origin accepted")
	}
	if w = call(t, h, "POST", "/api/setup", map[string]string{}, nil, ""); w.Code != 409 {
		t.Fatal("setup repeated")
	}
}
func TestDraftIsNotPublishedSubscription(t *testing.T) {
	s := testServer(t)
	u := model.NewUser("phone")
	e := model.Endpoint{ID: "entry", Name: "entry", Protocol: "shadowsocks", Transport: "tcp", Security: "none", Host: "127.0.0.1", Port: 23456, Enabled: true, Password: "endpoint-secret"}
	u.EndpointIDs = []string{e.ID}
	s.state.Endpoints = []model.Endpoint{e}
	s.state.Users = []model.User{u}
	h := s.PublicHandler()
	if w := call(t, h, "GET", "/s/"+u.Token, nil, nil, ""); w.Code != 404 {
		t.Fatal("draft user exposed")
	}
	s.applied = model.Clone(s.state)
	if w := call(t, h, "GET", "/s/"+u.Token, nil, nil, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	s.state.Users[0].Enabled = false
	if w := call(t, h, "GET", "/s/"+u.Token, nil, nil, ""); w.Code != 404 {
		t.Fatal("disabled link remained live")
	}
}
func TestBackupTampering(t *testing.T) {
	b, err := protect([]byte("credentials"), "long-backup-password")
	if err != nil {
		t.Fatal(err)
	}
	p, err := unprotect(b, "long-backup-password")
	if err != nil || string(p) != "credentials" {
		t.Fatal("roundtrip failed")
	}
	if _, err = unprotect(b, "another-password-123"); err == nil {
		t.Fatal("wrong password accepted")
	}
	b[len(b)-1] ^= 1
	if _, err = unprotect(b, "long-backup-password"); err == nil {
		t.Fatal("tampered backup accepted")
	}
}
