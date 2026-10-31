package server

import (
	"fmt"
	"net"
	"net/url"
	"prismflow/internal/model"
	"sort"
	"strconv"
	"strings"
)

type proxyRoute struct{ path, upstream string }
type proxySite struct {
	plain    bool
	host     string
	port     int
	upstream string
	routes   []proxyRoute
}
type accessConfiguration struct {
	Caddy       string   `json:"caddy"`
	Nginx       string   `json:"nginx"`
	Environment string   `json:"environment"`
	Warnings    []string `json:"warnings"`
	Direct      []string `json:"direct"`
}

func upstreamAddress(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	if host == "" || net.ParseIP(host).IsUnspecified() {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}
func buildAccessConfig(st model.State, o Options) accessConfiguration {
	result := accessConfiguration{Warnings: []string{}, Direct: []string{}}
	sites := map[string]*proxySite{}
	add := func(host string, port int) *proxySite {
		key := net.JoinHostPort(host, strconv.Itoa(port))
		if sites[key] == nil {
			sites[key] = &proxySite{host: host, port: port}
		}
		return sites[key]
	}
	for _, item := range []struct{ address, upstream string }{{st.Access.PanelURL, o.AdminListen}, {st.PublicURL, o.PublicListen}} {
		if item.address == "" {
			continue
		}
		if item.upstream == o.AdminListen && st.PublicURL != "" && normalizedOrigin(st.Access.PanelURL) == normalizedOrigin(st.PublicURL) {
			continue
		}
		u, err := url.Parse(item.address)
		if err != nil {
			continue
		}
		port := 443
		if u.Scheme == "http" {
			port = 80
		}
		if u.Port() != "" {
			port, _ = strconv.Atoi(u.Port())
		}
		upstream := upstreamAddress(item.upstream)
		if net.JoinHostPort(u.Hostname(), strconv.Itoa(port)) == upstream {
			result.Warnings = append(result.Warnings, "本机直连地址 "+item.address+" 无需反代；配置将跳过，避免回环转发。")
			continue
		}
		site := add(u.Hostname(), port)
		site.upstream = upstream
		site.plain = u.Scheme == "http"
	}
	for _, e := range st.Endpoints {
		if !e.Enabled {
			continue
		}
		if e.Native {
			result.Direct = append(result.Direct, e.Name+" · 原生监听器：按原生参数管理网络与端口")
			continue
		}
		if e.ExternalTLS() {
			site := add(e.Host, e.ClientPort())
			site.routes = append(site.routes, proxyRoute{e.Path, net.JoinHostPort("127.0.0.1", strconv.Itoa(e.Port))})
		} else {
			network := "TCP"
			for _, p := range model.Protocols {
				if p.ID == e.Protocol {
					network = strings.ToUpper(p.Network)
					break
				}
			}
			result.Direct = append(result.Direct, fmt.Sprintf("%s · %s:%d/%s → Mihomo", e.Name, e.Host, e.Port, network))
		}
	}
	keys := []string{}
	for key := range sites {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var caddy, nginx strings.Builder
	for _, key := range keys {
		site := sites[key]
		authority := net.JoinHostPort(site.host, strconv.Itoa(site.port))
		if site.plain {
			authority = "http://" + authority
		} else {
			authority = "https://" + authority
		}
		fmt.Fprintf(&caddy, "%s {\n", authority)
		for i, route := range site.routes {
			fmt.Fprintf(&caddy, "  @xhttp%d path %s %s/*\n  handle @xhttp%d {\n    reverse_proxy %s {\n      flush_interval -1\n    }\n  }\n", i, route.path, route.path, i, route.upstream)
		}
		if site.upstream != "" {
			fmt.Fprintf(&caddy, "  handle {\n    reverse_proxy %s\n  }\n", site.upstream)
		} else {
			caddy.WriteString("  handle {\n    respond 404\n  }\n")
		}
		caddy.WriteString("}\n\n")
		fmt.Fprintf(&nginx, "server {\n  listen %d%s;\n  server_name %s;\n", site.port, map[bool]string{true: " ssl http2", false: ""}[!site.plain], site.host)
		if !site.plain {
			fmt.Fprintf(&nginx, "  # 替换为反代维护的实际证书路径\n  ssl_certificate /etc/ssl/prismflow/%s/fullchain.pem;\n  ssl_certificate_key /etc/ssl/prismflow/%s/privkey.pem;\n  ssl_protocols TLSv1.2 TLSv1.3;\n", site.host, site.host)
		}
		for _, route := range site.routes {
			fmt.Fprintf(&nginx, "  location ~ ^%s(?:/|$) {\n    proxy_pass http://%s;\n    proxy_http_version 1.1;\n    proxy_set_header Host $host;\n    proxy_buffering off;\n    proxy_request_buffering off;\n    client_max_body_size 0;\n    proxy_read_timeout 3600s;\n    proxy_send_timeout 3600s;\n  }\n", route.path, route.upstream)
		}
		if site.upstream != "" {
			fmt.Fprintf(&nginx, "  location / {\n    proxy_pass http://%s;\n    proxy_set_header Host $http_host;\n    proxy_set_header X-Forwarded-Proto $scheme;\n  }\n", site.upstream)
		} else {
			nginx.WriteString("  location / { return 404; }\n")
		}
		nginx.WriteString("}\n\n")
	}
	// 面板签发的 HTTP-01 域名还需在已有 80 端口站点中合并该规则。
	challenges := []string{}
	for _, c := range st.Certificates {
		if c.Method == "http" {
			challenges = append(challenges, c.Domain)
		}
	}
	if len(challenges) > 0 {
		result.Warnings = append(result.Warnings, "面板管理的 HTTP-01 证书需要在对应域名的公网 80 端口合并挑战转发；已有反代证书优先使用系统路径，泛域名请选择 DNS-01。")
		fmt.Fprintf(&caddy, "# 合并到这些域名已有的 HTTP 站点；不要重复定义站点\nhttp://%s {\n  handle /.well-known/acme-challenge/* {\n    reverse_proxy %s\n  }\n  handle {\n    redir https://{host}{uri} 308\n  }\n}\n", strings.Join(challenges, ", http://"), upstreamAddress(o.PublicListen))
		fmt.Fprintf(&nginx, "# 合并到 %s 的现有 listen 80 server 内\n# location ^~ /.well-known/acme-challenge/ { proxy_pass http://%s; }\n", strings.Join(challenges, ", "), upstreamAddress(o.PublicListen))
	}
	for _, e := range st.Endpoints {
		if e.Enabled && !e.Native && !e.ExternalTLS() {
			for _, site := range sites {
				if e.Port == site.port && e.Transport != "quic" {
					result.Warnings = append(result.Warnings, fmt.Sprintf("%s 的直连 TCP %d 与反代监听端口冲突，请修改其中一个端口。", e.Name, e.Port))
					break
				}
			}
		}
	}
	if o.Origin != "" && st.Access.PanelURL != "" && o.Origin != st.Access.PanelURL {
		result.Warnings = append(result.Warnings, "PRISMFLOW_ORIGIN 环境变量优先于保存的面板地址，请更新服务环境后重启。")
	}
	for _, address := range []string{o.AdminListen, o.PublicListen} {
		host, _, err := net.SplitHostPort(address)
		if err == nil && !net.ParseIP(host).IsLoopback() {
			result.Warnings = append(result.Warnings, "当前 Web 服务未绑定回环地址；同机反代建议改用 127.0.0.1 并重启服务。")
		}
	}
	result.Caddy = caddy.String()
	result.Nginx = nginx.String()
	result.Environment = "PRISMFLOW_LISTEN=127.0.0.1:" + portOf(o.AdminListen) + "\nPRISMFLOW_PUBLIC_LISTEN=127.0.0.1:" + portOf(o.PublicListen) + "\n"
	if st.Access.PanelURL != "" {
		result.Environment += "PRISMFLOW_ORIGIN=" + st.Access.PanelURL + "\n"
	}
	return result
}
func portOf(address string) string { _, port, _ := net.SplitHostPort(address); return port }
