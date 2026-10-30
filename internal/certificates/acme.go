package certificates

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/log"
	"github.com/go-acme/lego/v4/registration"
	"io"
	stdlog "log"
	"net/http"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"prismflow/internal/store"
	"strings"
	"sync"
	"time"
)

type account struct {
	Email        string                 `json:"email"`
	Key          string                 `json:"key"`
	Registration *registration.Resource `json:"registration"`
	private      crypto.PrivateKey
}

func (a *account) GetEmail() string                        { return a.Email }
func (a *account) GetRegistration() *registration.Resource { return a.Registration }
func (a *account) GetPrivateKey() crypto.PrivateKey        { return a.private }

type Manager struct {
	Store      *store.Store
	Home       string
	mu         sync.Mutex
	challenges sync.Map
}

func (m *Manager) Present(domain, token, keyAuth string) error {
	m.challenges.Store(token, keyAuth)
	return nil
}
func (m *Manager) CleanUp(domain, token, keyAuth string) error {
	m.challenges.Delete(token)
	return nil
}
func (m *Manager) Challenge(w http.ResponseWriter, r *http.Request) {
	v, ok := m.challenges.Load(r.PathValue("token"))
	if !ok {
		if m.Home != "" {
			m.serveWebroot(w, r)
			return
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = io.WriteString(w, v.(string))
}
func (m *Manager) Issue(c model.Certificate) (model.Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !core.ValidCertificateDomain(c.Domain) || strings.HasPrefix(c.Domain, "*.") && c.Method != "cloudflare" || c.Email == "" {
		return c, errors.New("请输入域名和 ACME 联系邮箱")
	}
	if c.Engine == "acmesh" {
		return m.issueAcmeSH(c)
	}
	key := "acme-account:" + c.Email
	if c.Staging {
		key += ":staging"
	}
	a := account{Email: c.Email}
	err := m.Store.Get(key, &a)
	if errors.Is(err, sql.ErrNoRows) {
		k, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return c, e
		}
		der, e := x509.MarshalECPrivateKey(k)
		if e != nil {
			return c, e
		}
		a.Key = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
		a.private = k
	} else if err != nil {
		return c, err
	} else {
		block, _ := pem.Decode([]byte(a.Key))
		if block == nil {
			return c, errors.New("ACME 账户密钥损坏")
		}
		a.private, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return c, err
		}
	}
	// 禁止第三方库把挑战值或账户信息写入日志。
	log.Logger = stdlog.New(io.Discard, "", 0)
	config := lego.NewConfig(&a)
	config.Certificate.KeyType = certcrypto.EC256
	config.HTTPClient = &http.Client{Timeout: 45 * time.Second}
	if c.Staging {
		config.CADirURL = lego.LEDirectoryStaging
	}
	client, err := lego.NewClient(config)
	if err != nil {
		return c, err
	}
	switch c.Method {
	case "http":
		err = client.Challenge.SetHTTP01Provider(m)
	case "cloudflare":
		if c.DNSSecret == "" {
			return c, errors.New("缺少 Cloudflare API Token")
		}
		err = client.Challenge.SetDNS01Provider(&cloudflare{token: c.DNSSecret, records: map[string]record{}})
	default:
		return c, errors.New("不支持的 ACME 验证方式")
	}
	if err != nil {
		return c, err
	}
	if a.Registration == nil {
		a.Registration, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return c, errors.New("ACME 账户注册失败，请检查网络和邮箱")
		}
		if err = m.Store.Set(key, a); err != nil {
			return c, err
		}
	}
	resource, err := client.Certificate.Obtain(certificate.ObtainRequest{Domains: []string{c.Domain}, Bundle: true})
	if err != nil {
		return c, errors.New("ACME 签发失败：请检查域名解析、80 端口转发或 DNS Token 权限，并检查 CA 签发限制")
	}
	cert, err := core.ParseCertificate(string(resource.Certificate), string(resource.PrivateKey))
	if err != nil {
		return c, err
	}
	if err = core.CertificateCovers(cert, c.Domain); err != nil {
		return c, errors.New("签发证书未覆盖申请域名")
	}
	c.PEM = string(resource.Certificate)
	c.Key = string(resource.PrivateKey)
	c.ExpiresAt = cert.NotAfter
	c.LastError = ""
	return c, nil
}
