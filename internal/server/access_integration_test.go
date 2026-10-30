package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"strings"
	"testing"
	"time"
)

func accessTestPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}
func TestIntegrationExternalTLSReverseProxies(t *testing.T) {
	binary := os.Getenv("MIHOMO_BINARY")
	if binary == "" || os.Getenv("PRISMFLOW_REVERSE_PROXY_TEST") != "1" {
		t.Skip("设置 MIHOMO_BINARY 和 PRISMFLOW_REVERSE_PROXY_TEST=1，系统需要 caddy/nginx")
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer target.Close()
	for _, kind := range []string{"caddy", "nginx"} {
		t.Run(kind, func(t *testing.T) {
			tool, err := exec.LookPath(kind)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			tpl := &x509.Certificate{SerialNumber: big.NewInt(3), DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
			der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
			if err != nil {
				t.Fatal(err)
			}
			keyDER, err := x509.MarshalECPrivateKey(key)
			if err != nil {
				t.Fatal(err)
			}
			certPath := filepath.Join(dir, "cert.pem")
			keyPath := filepath.Join(dir, "key.pem")
			if err = os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
				t.Fatal(err)
			}
			e := model.Endpoint{ID: "reverse", Name: "External XHTTP", Protocol: "vless", Transport: "xhttp", Security: "tls", TLSMode: "external", Host: "127.0.0.1", ServerName: "127.0.0.1", Port: accessTestPort(t), PublicPort: accessTestPort(t), Path: "/reverse-test", Enabled: true}
			st := model.Default()
			u := model.NewUser("integration")
			u.EndpointIDs = []string{e.ID}
			st.Endpoints = []model.Endpoint{e}
			st.Users = []model.User{u}
			if err = core.ValidateReady(st, nil); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			manager := core.NewManager(binary, filepath.Join(dir, "server"), fmt.Sprintf("127.0.0.1:%d", accessTestPort(t)), model.Token(24))
			defer manager.Stop()
			config, err := core.Render(st, manager.Address, manager.Secret, "")
			if err != nil {
				t.Fatal(err)
			}
			if err = manager.Publish(ctx, config, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			generated := buildAccessConfig(st, Options{AdminListen: "127.0.0.1:8080", PublicListen: "127.0.0.1:8081"})
			var command *exec.Cmd
			var validate *exec.Cmd
			if kind == "caddy" {
				conf := fmt.Sprintf("{\n admin off\n auto_https off\n}\n") + generated.Caddy
				conf = strings.Replace(conf, fmt.Sprintf("https://127.0.0.1:%d {", e.PublicPort), fmt.Sprintf("https://127.0.0.1:%d {\n tls %s %s", e.PublicPort, certPath, keyPath), 1)
				path := filepath.Join(dir, "Caddyfile")
				if err = os.WriteFile(path, []byte(conf), 0600); err != nil {
					t.Fatal(err)
				}
				validate = exec.CommandContext(ctx, tool, "validate", "--config", path, "--adapter", "caddyfile")
				command = exec.CommandContext(ctx, tool, "run", "--config", path, "--adapter", "caddyfile")
			} else {
				conf := strings.ReplaceAll(generated.Nginx, "/etc/ssl/prismflow/127.0.0.1/fullchain.pem", certPath)
				conf = strings.ReplaceAll(conf, "/etc/ssl/prismflow/127.0.0.1/privkey.pem", keyPath)
				conf = fmt.Sprintf("daemon off;\nmaster_process off;\npid %s;\nerror_log stderr;\nevents {}\nhttp { access_log /dev/stderr; client_body_temp_path %s/body; proxy_temp_path %s/proxy; fastcgi_temp_path %s/fastcgi; uwsgi_temp_path %s/uwsgi; scgi_temp_path %s/scgi;\n%s\n}\n", filepath.Join(dir, "nginx.pid"), dir, dir, dir, dir, dir, conf)
				path := filepath.Join(dir, "nginx.conf")
				if err = os.WriteFile(path, []byte(conf), 0600); err != nil {
					t.Fatal(err)
				}
				validate = exec.CommandContext(ctx, tool, "-t", "-c", path, "-p", dir)
				command = exec.CommandContext(ctx, tool, "-c", path, "-p", dir)
			}
			if out, err := validate.CombinedOutput(); err != nil {
				t.Fatalf("generated configuration invalid: %v %s", err, out)
			}
			logFile, err := os.Create(filepath.Join(dir, "proxy.log"))
			if err != nil {
				t.Fatal(err)
			}
			defer logFile.Close()
			command.Stdout = logFile
			command.Stderr = logFile
			if err = command.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
			address := fmt.Sprintf("https://127.0.0.1:%d/not-a-node-path", e.PublicPort)
			cert, err := x509.ParseCertificate(der)
			if err != nil {
				t.Fatal(err)
			}
			roots := x509.NewCertPool()
			roots.AddCert(cert)
			httpClient := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}}}
			defer httpClient.CloseIdleConnections()
			ready := false
			for i := 0; i < 30; i++ {
				response, err := httpClient.Get(address)
				if err == nil {
					response.Body.Close()
					if response.StatusCode != 404 {
						t.Fatal("unmatched path did not return 404")
					}
					ready = true
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if !ready {
				logs, _ := os.ReadFile(logFile.Name())
				t.Fatalf("proxy not ready: %s", logs)
			}
			proxy := core.Proxy(e, u)
			hash := sha256.Sum256(der)
			proxy["fingerprint"] = hex.EncodeToString(hash[:])
			if _, err = core.CheckProxy(ctx, binary, dir, proxy, []string{target.URL}); err != nil {
				logs, _ := os.ReadFile(logFile.Name())
				t.Fatalf("real XHTTP through %s failed: %v\n%s", kind, err, logs)
			}
		})
	}
}
func TestIntegrationHTTPChallengeConfiguration(t *testing.T) {
	if os.Getenv("PRISMFLOW_REVERSE_PROXY_TEST") != "1" {
		t.Skip("需要 caddy")
	}
	st := model.Default()
	st.Certificates = []model.Certificate{{Domain: "node.example.com", Method: "http"}}
	config := buildAccessConfig(st, Options{PublicListen: "127.0.0.1:8081"})
	path := filepath.Join(t.TempDir(), "Caddyfile")
	if err := os.WriteFile(path, []byte(config.Caddy), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("caddy", "validate", "--config", path, "--adapter", "caddyfile").CombinedOutput(); err != nil {
		t.Fatalf("HTTP challenge configuration: %v %s", err, out)
	}
}
