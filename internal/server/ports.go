package server

import (
	"crypto/rand"
	"errors"
	"math/big"
	"net"
	"net/http"
	"prismflow/internal/model"
	"strconv"
)

func randomFreePort(used map[int]bool) (int, error) {
	for i := 0; i < 128; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(55536))
		if err != nil {
			return 0, err
		}
		port := 10000 + int(n.Int64())
		if used[port] {
			continue
		}
		addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(port))
		tcp, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		udp, err := net.ListenPacket("udp", addr)
		tcp.Close()
		if err != nil {
			continue
		}
		udp.Close()
		return port, nil
	}
	return 0, errors.New("无法分配空闲五位端口，请稍后重试")
}
func (s *Server) randomPorts(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Count int `json:"count"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Count < 1 || body.Count > 4 {
		fail(w, 400, errors.New("一次分配 1–4 个端口"))
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	used := map[int]bool{}
	for _, e := range s.state.Endpoints {
		used[e.Port] = true
	}
	for _, p := range s.reserved() {
		used[p] = true
	}
	ports := []int{}
	for i := 0; i < body.Count; i++ {
		p, err := randomFreePort(used)
		if err != nil {
			fail(w, 400, err)
			return
		}
		used[p] = true
		ports = append(ports, p)
	}
	respond(w, 200, ports)
}
func newEndpointDefaults(e *model.Endpoint, st model.State, reserved []int) error {
	if e.Host == "" {
		e.Host = st.DefaultHost
	}
	if e.Native {
		e.CertificateID = ""
		e.TLSMode = ""
		e.PublicPort = 0
	}
	if e.ExternalTLS() {
		if e.PublicPort == 0 {
			e.PublicPort = model.WebTLSPort(st.PublicURL)
		}
		e.CertificateID = ""
		e.ServerName = e.Host
	}
	if e.Security != "tls" {
		e.CertificateID = ""
	}
	if e.Security == "tls" && e.CertificateID == "" && !e.ExternalTLS() && !e.Native {
		e.CertificateID = st.DefaultCertificateID
	}
	if e.Port == 0 {
		used := map[int]bool{}
		for _, old := range st.Endpoints {
			used[old.Port] = true
		}
		for _, p := range reserved {
			used[p] = true
		}
		p, err := randomFreePort(used)
		if err != nil {
			return err
		}
		e.Port = p
	}
	if e.Port < 10000 || e.Port > 65535 {
		return errors.New("新入口需使用五位端口")
	}
	return nil
}
