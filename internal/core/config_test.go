package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"gopkg.in/yaml.v3"
	"math/big"
	"net"
	"prismflow/internal/model"
	"strings"
	"testing"
	"time"
)

func testCert(t *testing.T) model.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return model.Certificate{ID: "cert", Domain: "localhost", PEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), Key: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})), ExpiresAt: tpl.NotAfter, Method: "import"}
}
func testState(t *testing.T) model.State {
	t.Helper()
	s := model.Default()
	s.Certificates = []model.Certificate{testCert(t)}
	u := model.NewUser("测试用户")
	for i, p := range model.Protocols {
		e := model.Endpoint{Name: p.Name, Protocol: p.ID, Host: "127.0.0.1", Port: 21000 + i, Enabled: true, Transport: p.Transports[0], Security: p.Securities[0], CertificateID: "cert", ServerName: "localhost", RealityTarget: "localhost:443"}
		if err := model.InitEndpoint(&e); err != nil {
			t.Fatal(err)
		}
		s.Endpoints = append(s.Endpoints, e)
		u.EndpointIDs = append(u.EndpointIDs, e.ID)
	}
	s.Users = []model.User{u}
	return s
}
func TestRenderAndSubscription(t *testing.T) {
	s := testState(t)
	if err := Validate(s, []int{8080, 8081, 9097}); err != nil {
		t.Fatal(err)
	}
	b, err := Render(s, "127.0.0.1:9097", "test-secret", "http://127.0.0.1:8081")
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err = yaml.Unmarshal(b, &config); err != nil {
		t.Fatal(err)
	}
	if len(config["listeners"].([]any)) != 7 {
		t.Fatal("expected seven listeners")
	}
	sub, err := Subscription(s, s.Users[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sub), s.Endpoints[0].PrivateKey) || strings.Contains(string(sub), "PRIVATE KEY") {
		t.Fatal("subscription leaked server private key")
	}
	if len(strings.Split(Links(s, s.Users[0]), "\n")) != 7 {
		t.Fatal("expected seven links")
	}
}
func TestPortValidation(t *testing.T) {
	s := testState(t)
	s.Endpoints[1].Port = s.Endpoints[0].Port
	if err := Validate(s, nil); err != nil {
		t.Fatalf("TCP/UDP may coexist: %v", err)
	}
	s.Endpoints[2].Port = s.Endpoints[1].Port
	if err := Validate(s, nil); err == nil {
		t.Fatal("two UDP listeners collided")
	}
}
func TestInvalidCombinations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*model.State)
	}{{"reality websocket", func(s *model.State) { s.Endpoints[0].Transport = "ws"; s.Endpoints[0].Path = "/ws" }}, {"certificate mismatch", func(s *model.State) { s.Endpoints[1].ServerName = "invalid.example.com" }}, {"unknown assignment", func(s *model.State) { s.Users[0].EndpointIDs = append(s.Users[0].EndpointIDs, "unknown") }}, {"public plaintext", func(s *model.State) { s.PublicURL = "http://example.com" }}, {"reserved port", func(s *model.State) { s.Endpoints[0].Port = 8080 }}} {
		t.Run(tc.name, func(t *testing.T) {
			s := testState(t)
			tc.change(&s)
			if Validate(s, []int{8080}) == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
func TestDisabledUserHasNoListenerOrSubscription(t *testing.T) {
	s := testState(t)
	s.Users[0].Enabled = false
	b, err := Render(s, "127.0.0.1:9097", "secret", "http://127.0.0.1:8081")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "listeners: []") {
		t.Fatal("disabled user still exposes listeners")
	}
	if _, err = Subscription(s, s.Users[0]); err == nil {
		t.Fatal("disabled subscription remained active")
	}
}
