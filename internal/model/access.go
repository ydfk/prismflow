package model

import (
	"net/url"
	"strconv"
)

// HTTPS 入口的显式端口供新建反代 TLS 协议继承；既有协议保持原端口。
func WebTLSPort(address string) int {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "https" {
		return 443
	}
	port, err := strconv.Atoi(parsed.Port())
	if err == nil && port > 0 && port <= 65535 {
		return port
	}
	return 443
}

// 公网地址与实际监听地址分开保存；空字段兼容已有数据。
type AccessSettings struct {
	AdminPath  string
	BaseDomain string `json:"baseDomain"`
	PanelURL   string `json:"panelUrl"`
	XHTTPTLS   string `json:"xhttpTls"`
}

func (e Endpoint) ExternalTLS() bool { return e.TLSMode == "external" }
func (e Endpoint) ClientPort() int {
	if e.ExternalTLS() {
		if e.PublicPort != 0 {
			return e.PublicPort
		}
		return 443
	}
	return e.Port
}
func (e Endpoint) ListenHost() string {
	if e.ExternalTLS() {
		return "127.0.0.1"
	}
	return "0.0.0.0"
}

func (a AccessSettings) ManagementPath() string {
	if a.AdminPath == "" {
		return "/manage"
	}
	return a.AdminPath
}
