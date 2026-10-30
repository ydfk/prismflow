package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s *Server) secureCookie(r *http.Request) bool {
	if s.Options.SecureCookie || r.TLS != nil {
		return true
	}
	panel, _ := url.Parse(s.State().Access.PanelURL)
	return panel != nil && panel.Scheme == "https" && normalizedOrigin("https://"+panel.Host) == normalizedOrigin("https://"+r.Host)
}
func accessMarker(role string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		respond(w, 200, map[string]string{"service": "prismflow", "role": role})
	}
}

type accessCheck struct {
	Role    string   `json:"role"`
	Address string   `json:"address"`
	OK      bool     `json:"ok"`
	Message string   `json:"message"`
	IPs     []string `json:"ips"`
}

func checkAccessURL(ctx context.Context, role, address string) accessCheck {
	result := accessCheck{Role: role, Address: address, IPs: []string{}}
	if address == "" {
		result.Message = "尚未配置公网地址"
		return result
	}
	u, err := url.Parse(address)
	if err != nil || u.Host == "" {
		result.Message = "地址无效"
		return result
	}
	ips, err := net.DefaultResolver.LookupHost(ctx, u.Hostname())
	if err != nil {
		result.Message = "DNS 解析失败，请检查域名解析"
		return result
	}
	result.IPs = ips
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(address, "/")+"/.well-known/prismflow-access", nil)
	if err != nil {
		result.Message = "无法创建检查请求"
		return result
	}
	// 不跟随重定向，避免把登录页或另一个站点误认为正确路由。
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		result.Message = "连接或 TLS 验证失败，请检查反代、证书和防火墙"
		return result
	}
	defer response.Body.Close()
	var marker struct {
		Service string `json:"service"`
		Role    string `json:"role"`
	}
	err = json.NewDecoder(http.MaxBytesReader(nil, response.Body, 4096)).Decode(&marker)
	result.OK = response.StatusCode == 200 && err == nil && marker.Service == "prismflow" && marker.Role == role
	if result.OK {
		result.Message = "DNS、HTTPS 和服务路由正常（服务器视角）"
	} else {
		result.Message = "目标未返回对应服务标识，请核对反代上游；不要将订阅指向管理端口"
	}
	return result
}
func (s *Server) checkAccess(w http.ResponseWriter, r *http.Request) {
	state := s.State()
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	results := []accessCheck{checkAccessURL(ctx, "panel", state.Access.PanelURL), checkAccessURL(ctx, "public", state.PublicURL)}
	respond(w, 200, results)
}
func (s *Server) accessConfig(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, buildAccessConfig(s.State(), s.Options))
}
