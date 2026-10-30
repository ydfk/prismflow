package model

// 与锁定版 listener/parse.go 保持一致；原生模式不生成订阅凭据。
type ListenerType struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

var ListenerTypes = []ListenerType{
	{"vless", "VLESS", "UUID 认证；支持 TCP、WebSocket、gRPC、XHTTP、TLS 和 REALITY。"},
	{"vmess", "VMess", "UUID 认证，兼容 AEAD 客户端。"},
	{"trojan", "Trojan", "TLS 密码认证，支持多种传输。"},
	{"anytls", "AnyTLS", "TLS 会话复用，可设置填充方案。"},
	{"hysteria2", "Hysteria2", "基于 QUIC，使用 UDP 与 TLS。"},
	{"tuic", "TUIC", "QUIC 传输，支持 v4/v5 认证参数。"},
	{"shadowsocks", "Shadowsocks", "支持内核提供的加密方式与插件参数。"},
	{"socks", "SOCKS", "SOCKS 代理，支持用户名与密码认证。"},
	{"http", "HTTP", "HTTP 代理，可配置认证与 TLS。"},
	{"mixed", "Mixed", "同端口提供 HTTP 与 SOCKS 代理。"},
	{"snell", "Snell", "预共享密钥认证，版本取决于内核能力。"},
	{"mieru", "Mieru", "支持 TCP/UDP、用户与多路复用参数。"},
	{"sudoku", "Sudoku", "支持密钥与内核提供的编码参数。"},
	{"shadowquic", "ShadowQUIC", "QUIC 传输，需要对应的 TLS 与认证配置。"},
	{"hysteria2-realm", "Hysteria2 Realm", "Hysteria2 中继配置。"},
	{"trusttunnel", "TrustTunnel", "基于 TLS 的隧道，需要配置认证。"},
	{"tunnel", "Tunnel", "TCP/UDP 定向转发，需设置目标地址。"},
	{"redir", "Redirect", "透明代理，需系统转发规则。"},
	{"tproxy", "TProxy", "透明代理，需 Linux 权限与路由规则。"},
	{"tun", "TUN", "虚拟网卡，需设备、系统权限与路由配置。"},
}

func NativeSupported(id string) bool {
	for _, p := range ListenerTypes {
		if p.ID == id {
			return true
		}
	}
	return false
}
