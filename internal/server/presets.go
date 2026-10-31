package server

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"prismflow/internal/model"
	"strings"
)

type variant struct{ protocol, transport, security, label string }

var variants = map[string]variant{
	"reality":   {"vless", "tcp", "reality", "VLESS REALITY Vision"},
	"xhttp":     {"vless", "xhttp", "tls", "VLESS XHTTP TLS"},
	"anytls":    {"anytls", "tcp", "tls", "AnyTLS"},
	"hysteria2": {"hysteria2", "quic", "tls", "Hysteria2"},
}

func (s *Server) createPreset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Variants      []string       `json:"variants"`
		Host          string         `json:"host"`
		Ports         map[string]int `json:"ports"`
		CertificateID string         `json:"certificateId"`
		TLSMode       string         `json:"tlsMode"`
		PublicPort    int            `json:"publicPort"`
		RealityTarget string         `json:"realityTarget"`
		UserIDs       []string       `json:"userIds"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Variants) < 1 || len(in.Variants) > 4 {
		fail(w, 400, errors.New("请选择 1–4 项协议"))
		return
	}
	mutationResult(w, s.mutate(func(st *model.State) error {
		if in.Host == "" {
			in.Host = st.DefaultHost
		}
		if in.CertificateID == "" {
			in.CertificateID = st.DefaultCertificateID
		}
		for _, id := range in.UserIDs {
			found := false
			for _, u := range st.Users {
				if u.ID == id {
					found = true
				}
			}
			if !found {
				return errors.New("用户不存在")
			}
		}
		seen := map[string]bool{}
		used := map[int]bool{}
		for _, e := range st.Endpoints {
			used[e.Port] = true
		}
		for _, p := range s.reserved() {
			used[p] = true
		}
		for _, key := range in.Variants {
			v, ok := variants[key]
			if !ok || seen[key] {
				return errors.New("协议选择无效或重复")
			}
			seen[key] = true
			port := in.Ports[key]
			if port == 0 {
				var err error
				port, err = randomFreePort(used)
				if err != nil {
					return err
				}
			}
			if port < 10000 || port > 65535 || used[port] {
				return errors.New("请选择未占用的五位端口，或重新随机")
			}
			used[port] = true
			e := model.Endpoint{Name: fmt.Sprintf("%s · %d", v.label, port), Protocol: v.protocol, Transport: v.transport, Security: v.security, Host: in.Host, Port: port, Enabled: true, CertificateID: in.CertificateID, RealityTarget: in.RealityTarget}
			if e.Transport == "xhttp" {
				e.TLSMode = in.TLSMode
				e.PublicPort = in.PublicPort
				if e.ExternalTLS() {
					if e.PublicPort == 0 {
						e.PublicPort = model.WebTLSPort(st.PublicURL)
					}
					e.CertificateID = ""
				}
			}
			if e.Security != "tls" {
				e.CertificateID = ""
			}
			if e.Security == "reality" {
				e.ServerName, _, _ = net.SplitHostPort(in.RealityTarget)
			} else {
				for _, c := range st.Certificates {
					if c.ID == in.CertificateID {
						e.ServerName = c.Domain
						if strings.HasPrefix(c.Domain, "*.") {
							e.ServerName = e.Host
						}
					}
				}
			}
			if e.Transport == "xhttp" {
				e.Path = "/" + model.ID()
				if e.ExternalTLS() {
					e.ServerName = e.Host
				}
			}
			if err := model.InitEndpoint(&e); err != nil {
				return err
			}
			st.Endpoints = append(st.Endpoints, e)
			for i := range st.Users {
				for _, id := range in.UserIDs {
					if st.Users[i].ID == id {
						st.Users[i].EndpointIDs = append(st.Users[i].EndpointIDs, e.ID)
					}
				}
			}
		}
		return nil
	}))
}
