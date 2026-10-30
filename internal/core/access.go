package core

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"prismflow/internal/model"
	"regexp"
	"strconv"
	"strings"
)

var proxyPath = regexp.MustCompile(`^/[A-Za-z0-9_-]+(?:/[A-Za-z0-9_-]+)*$`)

func ValidPublicURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil {
		return false
	}
	if u.Port() != "" {
		port, e := strconv.Atoi(u.Port())
		if e != nil || port < 1 || port > 65535 {
			return false
		}
	}
	return u.Host != "" && ValidHost(u.Hostname()) && u.User == nil && u.RawQuery == "" && u.Fragment == "" && (u.Path == "" || u.Path == "/") && (u.Scheme == "https" || u.Scheme == "http" && (u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback()))
}
func ValidateAccess(s model.State) error {
	a := s.Access
	if a.BaseDomain != "" && (!ValidHost(a.BaseDomain) || net.ParseIP(a.BaseDomain) != nil || strings.HasPrefix(a.BaseDomain, "*.")) {
		return errors.New("基础域名请填写 example.com，不含 * 或协议前缀")
	}
	if a.PanelURL != "" && !ValidPublicURL(a.PanelURL) {
		return errors.New("面板地址须为 HTTPS 根地址；仅本机可用 HTTP")
	}
	if a.PanelURL != "" && s.PublicURL != "" && publicAuthority(a.PanelURL) == publicAuthority(s.PublicURL) {
		return errors.New("面板和订阅请使用不同的域名或端口")
	}
	if a.XHTTPTLS != "" && a.XHTTPTLS != "external" && a.XHTTPTLS != "core" {
		return errors.New("XHTTP TLS 默认方式无效")
	}
	paths := map[string][]string{}
	for _, e := range s.Endpoints {
		if e.TLSMode != "" && e.TLSMode != "core" && e.TLSMode != "external" {
			return errors.New("TLS 终止方式无效")
		}
		if !e.ExternalTLS() {
			if e.PublicPort != 0 {
				return errors.New("直连入口不能设置不同的公网端口")
			}
			continue
		}
		if e.Native || e.Protocol != "vless" || e.Transport != "xhttp" || e.Security != "tls" {
			return errors.New("外部 TLS 仅支持表单 VLESS + XHTTP + TLS")
		}
		if e.ClientPort() == e.Port {
			return errors.New("XHTTP 公网端口与本机监听端口必须不同")
		}
		if e.PublicPort < 0 || e.PublicPort > 65535 || !proxyPath.MatchString(e.Path) {
			return errors.New("外部 TLS 需要有效的公网端口和独立路径，如 /prismflow；路径仅含字母、数字、下划线或连字符")
		}
		if e.CertificateID != "" {
			return errors.New("外部 TLS 的证书由反代管理，无需绑定 Mihomo 证书")
		}
		if e.ServerName != "" && !strings.EqualFold(e.ServerName, e.Host) {
			return errors.New("外部 TLS 的 SNI 必须与公网节点域名一致，以匹配反代路由")
		}
		if !e.Enabled {
			continue
		}
		key := net.JoinHostPort(strings.ToLower(e.Host), fmt.Sprint(e.ClientPort()))
		for _, path := range paths[key] {
			if path == e.Path || strings.HasPrefix(path, e.Path+"/") || strings.HasPrefix(e.Path, path+"/") {
				return errors.New("同一公网节点的 XHTTP 路径不能相同或互相包含")
			}
		}
		paths[key] = append(paths[key], e.Path)
	}
	return nil
}

// 泛域名标签用于展示和申请，不能直接作为客户端 SNI。
func ValidCertificateDomain(domain string) bool {
	host := strings.TrimPrefix(domain, "*.")
	return ValidHost(host) && net.ParseIP(host) == nil && strings.Contains(host, ".")
}
func CertificateCovers(cert *x509.Certificate, domain string) error {
	if strings.HasPrefix(domain, "*.") {
		for _, san := range cert.DNSNames {
			if strings.EqualFold(san, domain) {
				return nil
			}
		}
		return errors.New("证书不包含该泛域名")
	}
	return cert.VerifyHostname(domain)
}
func CertificateNames(cert *x509.Certificate) []string {
	names := append([]string{}, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		names = append(names, ip.String())
	}
	return names
}
func publicAuthority(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return address
	}
	port := u.Port()
	if port == "" {
		port = "443"
		if u.Scheme == "http" {
			port = "80"
		}
	}
	return net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}
