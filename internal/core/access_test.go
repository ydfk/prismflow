package core

import (
	"crypto/x509"
	"net/url"
	"prismflow/internal/model"
	"strings"
	"testing"
)

func TestExternalTLSSeparatesPublicAndPrivatePorts(t *testing.T) {
	st := model.Default()
	e := model.Endpoint{ID: "external", Name: "XHTTP", Protocol: "vless", Transport: "xhttp", Security: "tls", TLSMode: "external", Host: "node.example.com", ServerName: "node.example.com", Port: 24567, PublicPort: 443, Path: "/flow", Enabled: true}
	u := model.NewUser("设备")
	u.EndpointIDs = []string{e.ID}
	st.Endpoints = []model.Endpoint{e}
	st.Users = []model.User{u}
	if err := ValidateReady(st, nil); err != nil {
		t.Fatal(err)
	}
	config, err := Render(st, "127.0.0.1:9097", "secret", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"listen: 127.0.0.1", "port: 24567", "allow-insecure: true"} {
		if !strings.Contains(string(config), part) {
			t.Fatal(string(config))
		}
	}
	if strings.Contains(string(config), "certificate:") {
		t.Fatal("external TLS contains core certificate")
	}
	proxy := Proxy(e, u)
	if proxy["port"] != 443 || proxy["tls"] != true {
		t.Fatal(proxy)
	}
	link, err := url.Parse(Share(e, u))
	if err != nil || link.Port() != "443" || link.Query().Get("security") != "tls" {
		t.Fatal(link, err)
	}
	for _, change := range []func(*model.Endpoint){func(e *model.Endpoint) { e.Protocol = "anytls" }, func(e *model.Endpoint) { e.ServerName = "other.example.com" }, func(e *model.Endpoint) { e.Path = "/bad;config" }, func(e *model.Endpoint) { e.CertificateID = "unexpected" }} {
		broken := model.Clone(st)
		change(&broken.Endpoints[0])
		if Validate(broken, nil) == nil {
			t.Fatal("invalid external TLS accepted")
		}
	}
	st.Endpoints = append(st.Endpoints, e)
	st.Endpoints[1].ID = "other"
	st.Endpoints[1].Name = "other"
	st.Endpoints[1].Port++
	if Validate(st, nil) == nil {
		t.Fatal("duplicate proxy path accepted")
	}
}
func TestWildcardCertificateCoverage(t *testing.T) {
	c := &x509.Certificate{DNSNames: []string{"*.example.com", "example.com"}}
	for _, name := range []string{"*.example.com", "node.example.com", "example.com"} {
		if err := CertificateCovers(c, name); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"nested.node.example.com", "*.other.com"} {
		if CertificateCovers(c, name) == nil {
			t.Fatal("wrong coverage", name)
		}
	}
}
func TestPublicAddressValidation(t *testing.T) {
	for _, value := range []string{"https://panel.example.com/path", "https://panel.example.com:99999", "http://panel.example.com", "https://user:secret@panel.example.com"} {
		if ValidPublicURL(value) {
			t.Fatal("unsafe or unsupported URL accepted", value)
		}
	}
	st := model.Default()
	st.Access.PanelURL = "https://PANEL.example.com:443"
	st.PublicURL = "https://panel.example.com"
	if err := ValidateAccess(st); err != nil {
		t.Fatal("shared domain rejected", err)
	}
	for _, path := range []string{"/", "/s", "/api", "/bad?path", "/manage/"} {
		st.Access.AdminPath = path
		if ValidateAccess(st) == nil {
			t.Fatal("invalid management path accepted", path)
		}
	}
}
