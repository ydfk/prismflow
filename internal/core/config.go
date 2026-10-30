package core

import (
	"crypto/ecdh"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"net"
	"net/url"
	"prismflow/internal/model"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var hostname = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

func ValidHost(s string) bool { return net.ParseIP(s) != nil || hostname.MatchString(s) }
func Validate(s model.State, reserved []int) error {
	if err := s.Traffic.Validate(); err != nil {
		return err
	}
	if s.Website.Disabled {
		for _, e := range s.Endpoints {
			if e.Enabled && !e.Native && e.Masquerade {
				return errors.New("伪装网站未启用，请启用网站或关闭入口的网站回落")
			}
		}
	}
	if s.DefaultHost != "" && !ValidHost(s.DefaultHost) {
		return errors.New("默认域名或地址无效")
	}
	if s.DefaultCertificateID != "" {
		found := false
		for _, c := range s.Certificates {
			if c.ID == s.DefaultCertificateID {
				found = true
			}
		}
		if !found {
			return errors.New("默认证书不存在")
		}
	}
	if len(s.Endpoints) > 100 || len(s.Users) > 1000 {
		return errors.New("最多支持 100 个入口和 1000 个订阅用户")
	}
	if s.RetentionDays < 1 || s.RetentionDays > 365 {
		return errors.New("历史保留天数必须在 1–365 之间")
	}
	if s.PublicURL != "" {
		u, err := url.Parse(s.PublicURL)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback()))) {
			return errors.New("订阅地址须为 HTTPS 根地址；仅本机可用 HTTP")
		}
	}
	certs := map[string]model.Certificate{}
	ids := map[string]bool{}
	names := map[string]bool{}
	ports := map[string]bool{}
	for _, c := range s.Certificates {
		if !safeID.MatchString(c.ID) || certs[c.ID].ID != "" {
			return errors.New("证书 ID 无效或重复")
		}
		certs[c.ID] = c
	}
	for _, e := range s.Endpoints {
		if !safeID.MatchString(e.ID) || ids[e.ID] {
			return errors.New("入口 ID 无效或重复")
		}
		ids[e.ID] = true
		if strings.TrimSpace(e.Name) == "" || len(e.Name) > 100 || names[e.Name] {
			return errors.New("入口名称不能为空、过长或重复")
		}
		names[e.Name] = true
		var p *model.Protocol
		for i := range model.Protocols {
			if model.Protocols[i].ID == e.Protocol {
				p = &model.Protocols[i]
			}
		}
		if e.Native {
			if _, err := NativeListener(e); err != nil {
				return fmt.Errorf("%s：%w", e.Name, err)
			}
			p = &model.Protocol{Network: "tcp+udp"}
		} else if p == nil || !slices.Contains(p.Transports, e.Transport) || !slices.Contains(p.Securities, e.Security) {
			return fmt.Errorf("%s：协议、传输或加密组合不支持", e.Name)
		}
		if !ValidHost(e.Host) {
			return fmt.Errorf("%s：请输入合法的服务器 IP 或域名", e.Name)
		}
		if e.Port < 1 || e.Port > 65535 || slices.Contains(reserved, e.Port) {
			return fmt.Errorf("%s：端口无效或被面板、网站、控制器保留", e.Name)
		}
		for _, network := range strings.Split(p.Network, "+") {
			key := network + strconv.Itoa(e.Port)
			if e.Enabled && ports[key] {
				return fmt.Errorf("%s：%s 端口 %d 冲突", e.Name, network, e.Port)
			}
			if e.Enabled {
				ports[key] = true
			}
		}
		if e.Native {
			continue
		}
		if (e.Transport == "ws" || e.Transport == "xhttp") && (!strings.HasPrefix(e.Path, "/") || len(e.Path) > 200) {
			return errors.New("WebSocket 路径必须以 / 开头，且不超过 200 字符")
		}
		if e.Transport == "grpc" && (!safeID.MatchString(e.Path)) {
			return errors.New("gRPC 服务名仅允许字母、数字、连字符、下划线")
		}
		if e.Security == "reality" {
			if e.Transport != "tcp" {
				return errors.New("第一版 REALITY 仅开放 TCP + Vision 组合")
			}
			host, port, err := net.SplitHostPort(e.RealityTarget)
			n, _ := strconv.Atoi(port)
			if err != nil || !ValidHost(host) || n < 1 || n > 65535 || !ValidHost(e.ServerName) {
				return errors.New("REALITY 需要合法的握手目标 host:port 和服务器名称")
			}
			b, err := base64.RawURLEncoding.DecodeString(e.PrivateKey)
			if err != nil {
				return errors.New("REALITY 私钥无效")
			}
			key, err := ecdh.X25519().NewPrivateKey(b)
			if err != nil || base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()) != e.PublicKey {
				return errors.New("REALITY 密钥对不匹配")
			}
		}
		if e.Security == "tls" {
			c, ok := certs[e.CertificateID]
			if !ok {
				return fmt.Errorf("%s：请选择证书", e.Name)
			}
			cert, err := ParseCertificate(c.PEM, c.Key)
			if err != nil {
				return err
			}
			name := e.ServerName
			if name == "" {
				name = e.Host
			}
			if err = cert.VerifyHostname(name); err != nil {
				return fmt.Errorf("%s：证书与服务器名称不匹配", e.Name)
			}
		}
		if e.Masquerade && e.Protocol != "hysteria2" {
			return errors.New("网站回落目前只支持 Hysteria2；REALITY 使用其握手目标")
		}
	}
	for _, e := range s.Endpoints {
		if e.Native {
			for _, u := range s.Users {
				if slices.Contains(u.EndpointIDs, e.ID) {
					return errors.New("原生监听器不能加入用户订阅")
				}
			}
		}
	}
	userIDs := map[string]bool{}
	for _, u := range s.Users {
		if !safeID.MatchString(u.ID) || userIDs[u.ID] || strings.TrimSpace(u.Name) == "" || len(u.Name) > 100 {
			return errors.New("用户 ID 或名称无效")
		}
		userIDs[u.ID] = true
		if len(u.Token) < 32 || len(u.Password) < 16 || len(u.UUID) != 36 {
			return errors.New("用户凭据无效")
		}
		for _, id := range u.EndpointIDs {
			if !ids[id] {
				return errors.New("用户关联了不存在的入口")
			}
		}
	}
	if len(s.Website.Title) > 100 || len(s.Website.Description) > 1000 || !slices.Contains([]string{"journal", "studio"}, s.Website.Template) {
		return errors.New("网站标题、介绍或模板无效")
	}
	return nil
}
func ValidateReady(s model.State, reserved []int) error {
	if err := Validate(s, reserved); err != nil {
		return err
	}
	for _, e := range s.Endpoints {
		if !e.Enabled || e.Native || e.Security != "tls" {
			continue
		}
		for _, c := range s.Certificates {
			if c.ID == e.CertificateID {
				cert, err := ParseCertificate(c.PEM, c.Key)
				if err != nil {
					return err
				}
				if time.Now().After(cert.NotAfter) {
					return fmt.Errorf("%s：证书已过期，请先续期或停用入口", e.Name)
				}
			}
		}
	}
	return nil
}
func ParseCertificate(certPEM, keyPEM string) (*x509.Certificate, error) {
	if _, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM)); err != nil {
		return nil, errors.New("证书或私钥无效，或两者不匹配")
	}
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, errors.New("无法解析 PEM 证书")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	if time.Now().Before(cert.NotBefore) {
		return nil, errors.New("证书尚未生效")
	}
	return cert, nil
}
func Render(s model.State, controller, secret, websiteURL string) ([]byte, error) {
	listeners := []map[string]any{}
	for _, e := range s.Endpoints {
		if !e.Enabled {
			continue
		}
		if e.Native {
			l, err := NativeListener(e)
			if err != nil {
				return nil, err
			}
			listeners = append(listeners, l)
			continue
		}
		assigned := []model.User{}
		for _, u := range s.Users {
			if model.Assigned(u, e) {
				assigned = append(assigned, u)
			}
		}
		if len(assigned) == 0 {
			continue
		}
		l := map[string]any{"name": "pf-" + e.ID, "type": e.Protocol, "listen": "0.0.0.0", "port": e.Port}
		switch e.Protocol {
		case "shadowsocks":
			l["cipher"] = "chacha20-ietf-poly1305"
			l["password"] = e.Password
			l["udp"] = true
		case "vless", "vmess", "trojan":
			users := []map[string]any{}
			for _, u := range assigned {
				v := map[string]any{"username": u.ID}
				if e.Protocol == "trojan" {
					v["password"] = u.Password
				} else {
					v["uuid"] = u.UUID
				}
				if e.Protocol == "vless" && e.Transport == "tcp" {
					v["flow"] = "xtls-rprx-vision"
				}
				users = append(users, v)
			}
			l["users"] = users
		default:
			users := map[string]string{}
			for _, u := range assigned {
				key := u.ID
				if e.Protocol == "tuic" {
					key = u.UUID
				}
				users[key] = u.Password
			}
			l["users"] = users
		}
		if e.Security == "tls" {
			for _, c := range s.Certificates {
				if c.ID == e.CertificateID {
					l["certificate"] = c.PEM
					l["private-key"] = c.Key
				}
			}
		}
		if e.Security == "reality" {
			l["reality-config"] = map[string]any{"dest": e.RealityTarget, "private-key": e.PrivateKey, "short-id": []string{e.ShortID}, "server-names": []string{e.ServerName}}
		}
		if e.Transport == "ws" {
			l["ws-path"] = e.Path
		}
		if e.Transport == "xhttp" {
			l["xhttp-config"] = map[string]any{"path": e.Path, "mode": "auto"}
		}
		if e.Transport == "grpc" {
			l["grpc-service-name"] = e.Path
		}
		if e.Protocol == "tuic" {
			l["congestion-controller"] = "bbr"
			l["alpn"] = []string{"h3"}
		}
		if e.Protocol == "hysteria2" && e.Masquerade {
			l["masquerade"] = websiteURL
		}
		listeners = append(listeners, l)
	}
	return yaml.Marshal(map[string]any{"mode": "rule", "log-level": "info", "ipv6": true, "allow-lan": false, "external-controller": controller, "secret": secret, "listeners": listeners, "rules": []string{"MATCH,DIRECT"}})
}
