package model

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

const CoreVersion = "v1.19.33"

type Endpoint struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Protocol      string `json:"protocol"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Enabled       bool   `json:"enabled"`
	Transport     string `json:"transport"`
	Path          string `json:"path"`
	Security      string `json:"security"`
	CertificateID string `json:"certificateId"`
	ServerName    string `json:"serverName"`
	RealityTarget string `json:"realityTarget"`
	PrivateKey    string `json:"privateKey"`
	PublicKey     string `json:"publicKey"`
	ShortID       string `json:"shortId"`
	Password      string `json:"password"`
	Masquerade    bool   `json:"masquerade"`
	Native        bool   `json:"native"`
	NativeConfig  string `json:"nativeConfig,omitempty"`
}

type User struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	EndpointIDs []string  `json:"endpointIds"`
	UUID        string    `json:"uuid"`
	Password    string    `json:"password"`
	Token       string    `json:"token"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Certificate struct {
	Engine    string    `json:"engine,omitempty"`
	CertPath  string    `json:"certPath,omitempty"`
	KeyPath   string    `json:"keyPath,omitempty"`
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	PEM       string    `json:"pem"`
	Key       string    `json:"key"`
	ExpiresAt time.Time `json:"expiresAt"`
	Method    string    `json:"method"`
	Email     string    `json:"email"`
	DNSSecret string    `json:"dnsSecret"`
	Renew     bool      `json:"renew"`
	Staging   bool      `json:"staging"`
	LastError string    `json:"lastError"`
}

type TrafficSettings struct {
	MaxBytes     int64  `json:"maxBytes"`
	ResetDay     int    `json:"resetDay"`
	Timezone     string `json:"timezone"`
	Interface    string `json:"interface"`
	OutboundOnly bool   `json:"outboundOnly"`
}
type Website struct {
	Disabled    bool   `json:"disabled"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Template    string `json:"template"`
	Bundle      string `json:"bundle"`
}

type State struct {
	Traffic              TrafficSettings `json:"traffic"`
	DefaultHost          string          `json:"defaultHost"`
	DefaultCertificateID string          `json:"defaultCertificateId"`
	Revision             int64           `json:"revision"`
	Endpoints            []Endpoint      `json:"endpoints"`
	Users                []User          `json:"users"`
	Certificates         []Certificate   `json:"certificates"`
	Website              Website         `json:"website"`
	PublicURL            string          `json:"publicUrl"`
	RetentionDays        int             `json:"retentionDays"`
}

func Default() State {
	return State{Endpoints: []Endpoint{}, Users: []User{}, Certificates: []Certificate{}, RetentionDays: 30, Website: Website{Title: "山与海之间", Description: "记录日常，探索世界。", Template: "journal"}}
}
func Clone(s State) State {
	b, _ := json.Marshal(s)
	var copy State
	_ = json.Unmarshal(b, &copy)
	return copy
}
func Token(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func ID() string { return Token(9) }
func UUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func NewUser(name string) User {
	return User{ID: ID(), Name: name, Enabled: true, EndpointIDs: []string{}, UUID: UUID(), Password: Token(24), Token: Token(32), CreatedAt: time.Now().UTC()}
}
func InitEndpoint(e *Endpoint) error {
	e.ID = ID()
	e.Password = Token(24)
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	e.PrivateKey = base64.RawURLEncoding.EncodeToString(key.Bytes())
	e.PublicKey = base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	b := make([]byte, 8)
	if _, err = rand.Read(b); err != nil {
		return err
	}
	e.ShortID = hex.EncodeToString(b)
	return nil
}
func Assigned(u User, e Endpoint) bool {
	return !e.Native && u.Enabled && e.Enabled && slices.Contains(u.EndpointIDs, e.ID)
}

type Protocol struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Summary    string   `json:"summary"`
	Detail     string   `json:"detail"`
	Transports []string `json:"transports"`
	Securities []string `json:"securities"`
	Network    string   `json:"network"`
}

var Protocols = []Protocol{
	{"vless", "VLESS", "灵活的 TCP 入口，支持 REALITY", "REALITY 无需自行签发证书，但需要合适的握手目标。TLS 支持 TCP、WebSocket 和 gRPC；Vision 仅用于 TCP。", []string{"tcp", "ws", "grpc", "xhttp"}, []string{"reality", "tls"}, "tcp"},
	{"hysteria2", "Hysteria2", "基于 QUIC，适合允许 UDP 的网络", "需要 TLS 证书和 UDP 端口；可将未认证的 HTTP/3 请求回落到伪装网站。实际速度取决于线路。", []string{"quic"}, []string{"tls"}, "udp"},
	{"tuic", "TUIC v5", "基于 QUIC 的多路传输", "需要支持 TUIC v5 的客户端、TLS 证书和 UDP 端口。与 Hysteria2 不能占用同一个 UDP 端口。", []string{"quic"}, []string{"tls"}, "udp"},
	{"trojan", "Trojan", "基于 TLS，客户端兼容面广", "需要 TLS 证书；支持 TCP、WebSocket 和 gRPC。本版本不承诺 Trojan 的网站回落。", []string{"tcp", "ws", "grpc"}, []string{"tls"}, "tcp"},
	{"anytls", "AnyTLS", "TLS 会话复用与填充", "需要 TLS 证书及支持 AnyTLS 的较新客户端；具体填充策略由内核默认值管理。", []string{"tcp"}, []string{"tls"}, "tcp"},
	{"shadowsocks", "Shadowsocks", "无需域名证书的轻量入口", "使用 chacha20-ietf-poly1305。Mihomo 此入站共享入口密码，多个订阅用户共用该入口时无法单独撤销凭据；重置入口密钥会影响所有使用者。", []string{"tcp"}, []string{"none"}, "tcp+udp"},
	{"vmess", "VMess", "兼容已有 VMess 客户端", "启用 AEAD（alterId=0）和 TLS，支持 TCP、WebSocket、gRPC；新配置可优先考虑 VLESS。", []string{"tcp", "ws", "grpc"}, []string{"tls"}, "tcp"},
}
