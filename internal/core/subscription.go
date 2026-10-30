package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"net"
	"net/url"
	"prismflow/internal/model"
	"strconv"
	"strings"
)

func Proxy(e model.Endpoint, u model.User) map[string]any {
	p := map[string]any{"name": SubscriptionName(e, u), "type": e.Protocol, "server": e.Host, "port": e.ClientPort(), "udp": true}
	name := e.ServerName
	if name == "" {
		name = e.Host
	}
	switch e.Protocol {
	case "vless", "vmess":
		p["uuid"] = u.UUID
		p["tls"] = true
		p["servername"] = name
		p["client-fingerprint"] = "chrome"
		if e.Protocol == "vmess" {
			p["alterId"] = 0
			p["cipher"] = "auto"
		} else if e.Transport == "tcp" {
			p["flow"] = "xtls-rprx-vision"
		}
	case "shadowsocks":
		p["type"] = "ss"
		p["cipher"] = "chacha20-ietf-poly1305"
		p["password"] = e.Password
	default:
		p["password"] = u.Password
		p["sni"] = name
	}
	if e.Protocol == "tuic" {
		p["uuid"] = u.UUID
		p["congestion-controller"] = "bbr"
		p["alpn"] = []string{"h3"}
	}
	if e.Security == "reality" {
		p["reality-opts"] = map[string]any{"public-key": e.PublicKey, "short-id": e.ShortID}
	}
	if e.Transport == "ws" {
		p["network"] = "ws"
		p["ws-opts"] = map[string]any{"path": e.Path}
	}
	if e.Transport == "xhttp" {
		p["network"] = "xhttp"
		p["xhttp-opts"] = map[string]any{"path": e.Path, "mode": "packet-up"}
	}
	if e.Transport == "grpc" {
		p["network"] = "grpc"
		p["grpc-opts"] = map[string]any{"grpc-service-name": e.Path}
	}
	return p
}
func Subscription(s model.State, u model.User) ([]byte, error) {
	p := []map[string]any{}
	names := []string{}
	for _, e := range s.Endpoints {
		if model.Assigned(u, e) {
			p = append(p, Proxy(e, u))
			names = append(names, SubscriptionName(e, u))
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("没有可用的协议入口，请联系管理员启用并分配入口")
	}
	return yaml.Marshal(map[string]any{"mixed-port": 7890, "allow-lan": false, "mode": "rule", "proxies": p, "proxy-groups": []any{map[string]any{"name": "PrismFlow", "type": "select", "proxies": names}}, "rules": []string{"MATCH,PrismFlow"}})
}
func Share(e model.Endpoint, u model.User) string {
	server := net.JoinHostPort(e.Host, strconv.Itoa(e.ClientPort()))
	name := e.ServerName
	if name == "" {
		name = e.Host
	}
	if e.Protocol == "vmess" {
		v := map[string]any{"v": "2", "ps": SubscriptionName(e, u), "add": e.Host, "port": strconv.Itoa(e.ClientPort()), "id": u.UUID, "aid": "0", "scy": "auto", "net": e.Transport, "type": "none", "host": name, "path": e.Path, "tls": "tls", "sni": name}
		b, _ := json.Marshal(v)
		return "vmess://" + base64.StdEncoding.EncodeToString(b)
	}
	if e.Protocol == "shadowsocks" {
		return "ss://" + base64.RawURLEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:"+e.Password)) + "@" + server + "#" + url.PathEscape(SubscriptionName(e, u))
	}
	q := url.Values{"sni": {name}}
	scheme := e.Protocol
	auth := url.User(u.Password)
	if e.Protocol == "vless" {
		auth = url.User(u.UUID)
		q.Set("encryption", "none")
		if e.Transport == "tcp" {
			q.Set("flow", "xtls-rprx-vision")
		}
	}
	if e.Protocol == "tuic" {
		auth = url.UserPassword(u.UUID, u.Password)
		q.Set("congestion_control", "bbr")
		q.Set("alpn", "h3")
	}
	if e.Transport == "tcp" || e.Transport == "ws" || e.Transport == "grpc" || e.Transport == "xhttp" {
		q.Set("security", e.Security)
		q.Set("type", e.Transport)
		if e.Transport == "ws" {
			q.Set("path", e.Path)
		}
		if e.Transport == "xhttp" {
			q.Set("path", e.Path)
			q.Set("mode", "packet-up")
		}
		if e.Transport == "grpc" {
			q.Set("serviceName", e.Path)
		}
	}
	if e.Security == "reality" {
		q.Set("pbk", e.PublicKey)
		q.Set("sid", e.ShortID)
		q.Set("fp", "chrome")
	}
	return (&url.URL{Scheme: scheme, User: auth, Host: server, RawQuery: q.Encode(), Fragment: SubscriptionName(e, u)}).String()
}
func Links(s model.State, u model.User) string {
	var lines []string
	for _, e := range s.Endpoints {
		if model.Assigned(u, e) {
			lines = append(lines, Share(e, u))
		}
	}
	return strings.Join(lines, "\n")
}
