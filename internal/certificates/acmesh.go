package certificates

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"runtime"
	"strings"
	"time"
)

const acmeArchiveURL = "https://codeload.github.com/acmesh-official/acme.sh/tar.gz/refs/tags/3.1.6"
const acmeArchiveHash = "0d3f9000ac44a6331314742a88c475f79134e24fc991997883652adc59efc486"

func (m *Manager) ensureAcmeSH(ctx context.Context) (string, error) {
	if runtime.GOOS == "windows" {
		return "", errors.New("acme.sh 需要 Linux；Windows 开发可选择内置 ACME 引擎")
	}
	if path := os.Getenv("PRISMFLOW_ACME_SH"); path != "" {
		if !filepath.IsAbs(path) {
			return "", errors.New("acme.sh 路径必须为绝对路径")
		}
		return path, nil
	}
	root := filepath.Join(m.Home, "tool")
	script := filepath.Join(root, "acme.sh")
	if _, err := os.Stat(script); err == nil {
		return script, nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", acmeArchiveURL, nil)
	if err != nil {
		return "", err
	}
	res, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return "", errors.New("无法下载 acme.sh，请检查服务器网络")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", errors.New("acme.sh 官方归档下载失败")
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 5<<20+1))
	if err != nil || len(b) > 5<<20 {
		return "", errors.New("acme.sh 归档过大或读取失败")
	}
	hash := sha256.Sum256(b)
	if hex.EncodeToString(hash[:]) != acmeArchiveHash {
		return "", errors.New("acme.sh SHA-256 校验失败")
	}
	if err = os.MkdirAll(m.Home, 0700); err != nil {
		return "", err
	}
	temp, err := os.MkdirTemp(m.Home, "tool-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	z, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	defer z.Close()
	tr := tar.NewReader(io.LimitReader(z, 20<<20))
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", e
		}
		parts := strings.SplitN(h.Name, "/", 2)
		if len(parts) != 2 || parts[1] == "" {
			continue
		}
		rel := filepath.Clean(filepath.FromSlash(parts[1]))
		if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", errors.New("acme.sh 归档路径异常")
		}
		dest := filepath.Join(temp, rel)
		if h.Typeflag == tar.TypeDir {
			if err = os.MkdirAll(dest, 0700); err != nil {
				return "", err
			}
			continue
		}
		if h.Typeflag != tar.TypeReg || h.Size > 2<<20 {
			return "", errors.New("acme.sh 归档条目不支持")
		}
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return "", err
		}
		f, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return "", e
		}
		_, e = io.Copy(f, tr)
		closeErr := f.Close()
		if e != nil {
			return "", e
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	if err = os.Rename(temp, root); err != nil {
		return "", err
	}
	return script, nil
}
func (m *Manager) issueAcmeSH(c model.Certificate) (model.Certificate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	script, err := m.ensureAcmeSH(ctx)
	if err != nil {
		return c, err
	}
	for _, name := range []string{"openssl", "curl", "sh"} {
		if _, err = exec.LookPath(name); err != nil {
			return c, errors.New("acme.sh 需要系统安装 openssl、curl 和 sh")
		}
	}
	if c.DNSSecret == "" && c.Method == "cloudflare" {
		return c, errors.New("缺少 Cloudflare API Token")
	}
	home := filepath.Join(m.Home, "accounts", "production")
	server := "letsencrypt"
	if c.Staging {
		home = filepath.Join(m.Home, "accounts", "staging")
		server = "letsencrypt_test"
	}
	out := filepath.Join(m.Home, "certificates", c.ID)
	webroot := filepath.Join(m.Home, "webroot")
	for _, dir := range []string{home, out, webroot} {
		if err = os.MkdirAll(dir, 0700); err != nil {
			return c, err
		}
	}
	base := []string{script, "--home", filepath.Dir(script), "--config-home", home, "--server", server, "--accountemail", c.Email}
	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "sh", append(append([]string{}, base...), args...)...)
		cmd.Env = append(os.Environ(), "CF_Token="+c.DNSSecret, "AUTO_UPGRADE=0")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			return errors.New("acme.sh 执行失败，请检查域名验证、证书权限及 CA 限制")
		}
		return nil
	}
	args := []string{"--issue", "-d", c.Domain, "--keylength", "ec-256", "--force"}
	if c.Method == "cloudflare" {
		args = append(args, "--dns", "dns_cf")
	} else {
		args = append(args, "--webroot", webroot)
	}
	if err = run(args...); err != nil {
		return c, err
	}
	certPath := filepath.Join(out, "fullchain.pem")
	keyPath := filepath.Join(out, "privkey.pem")
	if err = run("--install-cert", "-d", c.Domain, "--ecc", "--fullchain-file", certPath, "--key-file", keyPath); err != nil {
		return c, err
	}
	pem, err := os.ReadFile(certPath)
	if err != nil {
		return c, err
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return c, err
	}
	_ = os.Chmod(keyPath, 0600)
	cert, err := core.ParseCertificate(string(pem), string(key))
	if err != nil {
		return c, err
	}
	if err = core.CertificateCovers(cert, c.Domain); err != nil {
		return c, errors.New("签发证书未覆盖申请域名")
	}
	c.PEM = string(pem)
	c.Key = string(key)
	c.ExpiresAt = cert.NotAfter
	c.LastError = ""
	return c, nil
}
func (m *Manager) serveWebroot(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" || strings.ContainsAny(token, "./\\") {
		http.NotFound(w, r)
		return
	}
	b, err := os.ReadFile(filepath.Join(m.Home, "webroot", ".well-known", "acme-challenge", token))
	if err != nil || len(b) > 8192 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write(b)
}
