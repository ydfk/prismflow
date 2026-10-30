package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"prismflow/internal/model"
	"strings"
	"testing"
	"time"
)

func integrationBinary(t *testing.T) string {
	t.Helper()
	p := os.Getenv("MIHOMO_BINARY")
	if p == "" {
		t.Skip("设置 MIHOMO_BINARY 运行真实内核验证")
	}
	p, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func freePort(t *testing.T) int {
	t.Helper()
	for i := 0; i < 100; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(20000))
		if err != nil {
			t.Fatal(err)
		}
		port := 20000 + int(n.Int64())
		l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			continue
		}
		u, err := net.ListenPacket("udp", fmt.Sprintf("0.0.0.0:%d", port))
		l.Close()
		if err == nil {
			u.Close()
			return port
		}
	}
	t.Fatal("no available TCP/UDP port")
	return 0
}
func TestIntegrationGeneratedListeners(t *testing.T) {
	binary := integrationBinary(t)
	s := testState(t)
	for i := range s.Endpoints {
		s.Endpoints[i].Port = freePort(t)
	}
	address := fmt.Sprintf("127.0.0.1:%d", freePort(t))
	m := NewManager(binary, t.TempDir(), address, "integration-secret")
	t.Cleanup(m.Stop)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	b, err := Render(s, address, m.Secret, "http://127.0.0.1:8081")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Validate(ctx, b); err != nil {
		t.Fatal(err)
	}
	if err = m.Publish(ctx, b, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if !m.Running() {
		t.Fatal("core not running")
	}
	bad := []byte("listeners: [invalid\n")
	if err = m.Publish(ctx, bad, func() error { return nil }); err == nil {
		t.Fatal("invalid config accepted")
	}
	if !m.Running() {
		t.Fatal("failed validation stopped valid core")
	}
}
func TestIntegrationFailedPublishRestoresListeners(t *testing.T) {
	binary := integrationBinary(t)
	s := testState(t)
	s.Endpoints = s.Endpoints[5:6]
	s.Endpoints[0].Port = freePort(t)
	m := NewManager(binary, t.TempDir(), fmt.Sprintf("127.0.0.1:%d", freePort(t)), model.Token(24))
	t.Cleanup(m.Stop)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	original, err := Render(s, m.Address, m.Secret, "http://127.0.0.1:8081")
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Publish(ctx, original, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	occupied, err := occupiedListener()
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	if probe, probeErr := net.Listen("tcp", occupied.Addr().String()); probeErr == nil {
		probe.Close()
		t.Fatal("test socket does not block a second listener")
	}
	s.Endpoints[0].Port = occupied.Addr().(*net.TCPAddr).Port
	candidate, err := Render(s, m.Address, m.Secret, "http://127.0.0.1:8081")
	if err != nil {
		t.Fatal(err)
	}
	committed := false
	if err = m.Publish(ctx, candidate, func() error { committed = true; return nil }); err == nil || committed {
		t.Fatal("occupied listener was incorrectly committed")
	}
	if !m.Running() {
		t.Fatal("previous core was not restored")
	}
	active, err := os.ReadFile(filepath.Join(m.Dir, "active.yaml"))
	if err != nil || string(active) != string(original) {
		t.Fatal("previous config was not restored")
	}
	if err = m.Publish(ctx, original, func() error { return errors.New("database failure") }); err == nil {
		t.Fatal("commit failure ignored")
	}
	if !m.Running() {
		t.Fatal("commit failure did not restore previous core")
	}
}

func TestIntegrationProtocolRoundTrips(t *testing.T) {
	binary := integrationBinary(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "prismflow-roundtrip") }))
	defer target.Close()
	handshake := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "handshake-site") }))
	handshakeCert := testCert(t)
	pair, err := tls.X509KeyPair([]byte(handshakeCert.PEM), []byte(handshakeCert.Key))
	if err != nil {
		t.Fatal(err)
	}
	handshake.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS13}
	handshake.EnableHTTP2 = true
	handshake.StartTLS()
	defer handshake.Close()
	for _, combination := range []string{"shadowsocks/tcp", "trojan/tcp", "trojan/ws", "trojan/grpc", "vmess/tcp", "vmess/ws", "vmess/grpc", "hysteria2/quic", "tuic/quic", "anytls/tcp", "vless/tcp", "vless/ws", "vless/grpc", "vless/reality", "vless/xhttp"} {
		t.Run(combination, func(t *testing.T) {
			parts := strings.Split(combination, "/")
			kind, transport := parts[0], parts[1]
			s := testState(t)
			s.Endpoints = nil
			s.Users[0].EndpointIDs = nil
			e := model.Endpoint{Name: "test", Protocol: kind, Host: "127.0.0.1", Port: freePort(t), Enabled: true, Transport: transport, Security: "tls", CertificateID: "cert", ServerName: "localhost"}
			if transport == "ws" || transport == "xhttp" {
				e.Path = "/prismflow"
			}
			if transport == "grpc" {
				e.Path = "prismflow"
			}
			if transport == "reality" {
				e.Security = "reality"
				e.Transport = "tcp"
				e.RealityTarget = strings.TrimPrefix(handshake.URL, "https://")
			}
			if kind == "hysteria2" || kind == "tuic" {
				e.Transport = "quic"
			}
			if kind == "shadowsocks" {
				e.Security = "none"
			}
			if err := model.InitEndpoint(&e); err != nil {
				t.Fatal(err)
			}
			s.Endpoints = []model.Endpoint{e}
			s.Users[0].EndpointIDs = []string{e.ID}
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			server := NewManager(binary, t.TempDir(), fmt.Sprintf("127.0.0.1:%d", freePort(t)), model.Token(24))
			defer server.Stop()
			config, err := Render(s, server.Address, server.Secret, "http://127.0.0.1:8081")
			if err != nil {
				t.Fatal(err)
			}
			if err = server.Publish(ctx, config, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			proxy := Proxy(e, s.Users[0])
			// 测试只信任本次生成的证书指纹，不修改系统根证书，也不关闭校验。
			block, _ := pem.Decode([]byte(s.Certificates[0].PEM))
			fingerprint := sha256.Sum256(block.Bytes)
			if e.Security == "tls" {
				proxy["fingerprint"] = hex.EncodeToString(fingerprint[:])
			}
			client := NewManager(binary, t.TempDir(), fmt.Sprintf("127.0.0.1:%d", freePort(t)), model.Token(24))
			defer client.Stop()
			mixedPort := freePort(t)
			clientConfig, err := yaml.Marshal(map[string]any{"mixed-port": mixedPort, "bind-address": "127.0.0.1", "allow-lan": false, "mode": "rule", "external-controller": client.Address, "secret": client.Secret, "proxies": []any{proxy}, "rules": []string{"MATCH," + proxy["name"].(string)}})
			if err != nil {
				t.Fatal(err)
			}
			if err = client.Publish(ctx, clientConfig, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", mixedPort), 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer connection.Close()
			_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
			_, err = fmt.Fprintf(connection, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target.URL, strings.TrimPrefix(target.URL, "http://"))
			if err != nil {
				t.Fatal(err)
			}
			b, err := io.ReadAll(connection)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), "prismflow-roundtrip") {
				t.Fatalf("roundtrip failed for %s: %s", kind, string(b))
			}
		})
	}
}
