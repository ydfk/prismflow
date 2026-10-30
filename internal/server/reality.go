package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type targetResult struct {
	Target    string `json:"target"`
	Supported bool   `json:"supported"`
	LatencyMS int64  `json:"latencyMs"`
	TLS13     bool   `json:"tls13"`
	H2        bool   `json:"h2"`
	Message   string `json:"message"`
}

func publicTargetIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && !inCarrierRange(ip)
}
func inCarrierRange(ip net.IP) bool {
	v := ip.To4()
	return v != nil && v[0] == 100 && v[1] >= 64 && v[1] <= 127
}
func checkTargetTLS(ctx context.Context, address, host string, roots *x509.CertPool) targetResult {
	result := targetResult{}
	start := time.Now()
	conn, err := (&tls.Dialer{NetDialer: &net.Dialer{Timeout: 4 * time.Second}, Config: &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, ServerName: host, NextProtos: []string{"h2"}, RootCAs: roots}}).DialContext(ctx, "tcp", address)
	result.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		result.Message = "TLS 1.3 或证书校验未通过"
		return result
	}
	defer conn.Close()
	state := conn.(*tls.Conn).ConnectionState()
	result.TLS13 = state.Version == tls.VersionTLS13
	result.H2 = state.NegotiatedProtocol == "h2"
	result.Supported = result.TLS13 && result.H2
	if result.Supported {
		result.Message = "TLS 1.3 / h2 通过"
	} else {
		result.Message = "目标未协商 h2"
	}
	return result
}
func probeTarget(ctx context.Context, target string) targetResult {
	fail := targetResult{Target: target}
	host, port, err := net.SplitHostPort(target)
	n, _ := strconv.Atoi(port)
	if err != nil || host == "" || n < 1 || n > 65535 {
		fail.Message = "目标格式需为域名:端口"
		return fail
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		fail.Message = "域名解析失败"
		return fail
	}
	for _, a := range addresses {
		if !publicTargetIP(a.IP) {
			fail.Message = "仅检测公网目标"
			return fail
		}
	}
	// 固定已解析的地址建立 TLS，避免再次解析指向本机或内网。
	var result targetResult
	for i, a := range addresses {
		if i >= 2 {
			break
		}
		result = checkTargetTLS(ctx, net.JoinHostPort(a.IP.String(), port), host, nil)
		if result.Supported {
			break
		}
	}
	result.Target = target
	return result
}
func (s *Server) testRealityTargets(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Targets []string `json:"targets"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Targets) < 1 || len(in.Targets) > 4 {
		fail(w, 400, errors.New("一次检测 1–4 个目标"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()
	result := make([]targetResult, len(in.Targets))
	var wg sync.WaitGroup
	for _, target := range in.Targets {
		if len(target) > 260 {
			fail(w, 400, errors.New("目标过长"))
			return
		}
	}
	for i, target := range in.Targets {
		wg.Add(1)
		go func(i int, target string) { defer wg.Done(); result[i] = probeTarget(ctx, target) }(i, target)
	}
	wg.Wait()
	respond(w, 200, result)
}
