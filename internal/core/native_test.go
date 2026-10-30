package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"prismflow/internal/model"
	"strings"
	"testing"
	"time"
)

func TestNativeParameterBoundaries(t *testing.T) {
	e := model.Endpoint{ID: "native", Name: "native", Protocol: "mixed", Host: "localhost", Port: 23456, Native: true, Enabled: true, NativeConfig: "listen: 127.0.0.1\nusers: [{username: admin, password: secret}]"}
	st := model.Default()
	st.Endpoints = []model.Endpoint{e}
	if err := Validate(st, nil); err != nil {
		t.Fatal(err)
	}
	b, err := Render(st, "127.0.0.1:9097", "secret", "")
	if err != nil || !strings.Contains(string(b), "password: secret") {
		t.Fatal("native config not rendered", err)
	}
	for _, raw := range []string{"port: 22", "name: override", "type: http", "[invalid", "[]", "a: 1\n---\nb: 2"} {
		e.NativeConfig = raw
		if _, err := NativeListener(e); err == nil {
			t.Fatalf("accepted invalid raw config %q", raw)
		}
	}
	user := model.NewUser("test")
	user.EndpointIDs = []string{e.ID}
	st.Users = []model.User{user}
	if err := Validate(st, nil); err == nil {
		t.Fatal("native credentials mixed with subscription users")
	}
}
func TestIntegrationNativeMixedAuthentication(t *testing.T) {
	binary := integrationBinary(t)
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "native-proxy-ok") }))
	defer destination.Close()
	port := freePort(t)
	st := model.Default()
	st.Endpoints = []model.Endpoint{{ID: "native", Name: "native", Protocol: "mixed", Host: "127.0.0.1", Port: port, Enabled: true, Native: true, NativeConfig: "listen: 127.0.0.1\nusers: [{username: prismflow, password: native-test-secret}]"}}
	address := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	m := NewManager(binary, t.TempDir(), address, model.Token(24))
	defer m.Stop()
	b, err := Render(st, address, m.Secret, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Publish(context.Background(), b, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	proxy, _ := url.Parse(fmt.Sprintf("http://prismflow:native-test-secret@127.0.0.1:%d", port))
	transport := &http.Transport{Proxy: http.ProxyURL(proxy)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	response, err := client.Get(destination.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "native-proxy-ok" {
		t.Fatal("native proxy request failed", response.Status)
	}
	proxy.User = url.UserPassword("prismflow", "wrong")
	badTransport := &http.Transport{Proxy: http.ProxyURL(proxy)}
	defer badTransport.CloseIdleConnections()
	badClient := &http.Client{Transport: badTransport, Timeout: 5 * time.Second}
	response, err = badClient.Get(destination.URL)
	if err == nil {
		defer response.Body.Close()
		if response.StatusCode != http.StatusProxyAuthRequired && response.StatusCode != http.StatusForbidden {
			t.Fatalf("unexpected rejection status for invalid credentials: %d", response.StatusCode)
		}
	}
}
