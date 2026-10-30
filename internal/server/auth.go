package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"prismflow/internal/model"
	"strings"
	"time"
)

type adminAccount struct {
	Username string `json:"username"`
	Hash     string `json:"hash"`
}
type session struct {
	CSRF    string
	Expires time.Time
}
type attempt struct {
	Count int
	Until time.Time
}

func isMissing(err error) bool { return errors.Is(err, sql.ErrNoRows) }
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, err error) {
	respond(w, status, map[string]string{"error": err.Error()})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, errors.New("请求格式错误或内容过大"))
		return false
	}
	if d.Decode(&struct{}{}) != io.EOF {
		fail(w, 400, errors.New("请求不能包含多份 JSON"))
		return false
	}
	return true
}
func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
func (s *Server) sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	expected := s.Options.Origin
	if expected == "" {
		expected = s.State().Access.PanelURL
	}
	if expected == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		expected = scheme + "://" + r.Host
	}
	incoming := normalizedOrigin(origin)
	if incoming != "" && incoming == normalizedOrigin(strings.TrimRight(expected, "/")) {
		return true
	}
	// 设置公网域名后仍保留本机 SSH 隧道配置入口；显式 ORIGIN 不放宽。
	if s.Options.Origin == "" && r.TLS == nil && incoming != "" && incoming == normalizedOrigin("http://"+r.Host) {
		peer, _, _ := net.SplitHostPort(r.RemoteAddr)
		host, _, _ := net.SplitHostPort(r.Host)
		if net.ParseIP(peer).IsLoopback() && (host == "localhost" || net.ParseIP(host).IsLoopback()) {
			return true
		}
	}
	return false
}
func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	var account adminAccount
	err := s.Store.Get("admin", &account)
	if err != nil && !isMissing(err) {
		fail(w, 500, errors.New("无法读取管理员状态"))
		return
	}
	respond(w, 200, map[string]any{"setupRequired": isMissing(err), "name": "PrismFlow"})
}
func validPassword(p string) bool { return len(p) >= 12 && len(p) <= 72 }
func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		fail(w, 403, errors.New("来源不允许"))
		return
	}
	var body struct {
		Token    string `json:"token"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	s.authMu.Lock()
	defer s.authMu.Unlock()
	var admin adminAccount
	if err := s.Store.Get("admin", &admin); !isMissing(err) {
		fail(w, 409, errors.New("初始化已完成或管理员状态不可读"))
		return
	}
	if s.setupToken == "" || subtle.ConstantTimeCompare([]byte(body.Token), []byte(s.setupToken)) != 1 {
		fail(w, 403, errors.New("初始化令牌不正确，请读取服务器上的 setup.token"))
		return
	}
	if strings.TrimSpace(body.Username) == "" || len(body.Username) > 64 || !validPassword(body.Password) {
		fail(w, 400, errors.New("用户名不能为空；密码需为 12–72 字节"))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	if err == nil {
		err = s.Store.Set("admin", adminAccount{body.Username, string(hash)})
	}
	if err != nil {
		fail(w, 500, errors.New("无法保存管理员"))
		return
	}
	s.setupToken = ""
	_ = os.Remove(filepath.Join(s.Options.DataDir, "setup.token"))
	s.Store.Event("管理员初始化完成")
	respond(w, 201, map[string]bool{"ok": true})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.sameOrigin(r) {
		fail(w, 403, errors.New("来源不允许"))
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	s.authMu.Lock()
	defer s.authMu.Unlock()
	now := time.Now()
	for key, v := range s.attempts {
		if now.After(v.Until) {
			delete(s.attempts, key)
		}
	}
	a := s.attempts[ip]
	if a.Count >= 5 || len(s.attempts) > 1024 {
		fail(w, 429, errors.New("尝试过多，请 15 分钟后重试"))
		return
	}
	a.Count++
	a.Until = now.Add(15 * time.Minute)
	s.attempts[ip] = a
	var account adminAccount
	err := s.Store.Get("admin", &account)
	if err != nil || !validPassword(body.Password) || bcrypt.CompareHashAndPassword([]byte(account.Hash), []byte(body.Password)) != nil || body.Username != account.Username {
		fail(w, 401, errors.New("用户名或密码不正确"))
		return
	}
	delete(s.attempts, ip)
	for key, v := range s.sessions {
		if now.After(v.Expires) {
			delete(s.sessions, key)
		}
	}
	if len(s.sessions) >= 256 {
		fail(w, 429, errors.New("活动会话过多"))
		return
	}
	token := model.Token(32)
	sess := session{model.Token(24), now.Add(12 * time.Hour)}
	s.sessions[tokenHash(token)] = sess
	http.SetCookie(w, &http.Cookie{Name: "prismflow_session", Value: token, Path: "/", HttpOnly: true, Secure: s.secureCookie(r), SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	respond(w, 200, map[string]string{"csrf": sess.CSRF})
}
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("prismflow_session")
		if err != nil {
			fail(w, 401, errors.New("请先登录"))
			return
		}
		s.authMu.Lock()
		sess, ok := s.sessions[tokenHash(cookie.Value)]
		s.authMu.Unlock()
		if !ok || time.Now().After(sess.Expires) {
			fail(w, 401, errors.New("会话已过期，请重新登录"))
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if !s.sameOrigin(r) || subtle.ConstantTimeCompare([]byte(sess.CSRF), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
				fail(w, 403, errors.New("请求校验失败，请刷新页面"))
				return
			}
		}
		w.Header().Set("X-CSRF-Token", sess.CSRF)
		next.ServeHTTP(w, r)
	})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie("prismflow_session")
	s.authMu.Lock()
	if c != nil {
		delete(s.sessions, tokenHash(c.Value))
	}
	s.authMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "prismflow_session", Value: "", Path: "/", HttpOnly: true, Secure: s.secureCookie(r), SameSite: http.SameSiteStrictMode, MaxAge: -1})
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Current  string `json:"current"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	s.authMu.Lock()
	defer s.authMu.Unlock()
	var a adminAccount
	if s.Store.Get("admin", &a) != nil || bcrypt.CompareHashAndPassword([]byte(a.Hash), []byte(body.Current)) != nil {
		fail(w, 403, errors.New("当前密码不正确"))
		return
	}
	if !validPassword(body.Password) {
		fail(w, 400, errors.New("密码需为 12–72 字节"))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	if err == nil {
		a.Hash = string(hash)
		err = s.Store.Set("admin", a)
	}
	if err != nil {
		fail(w, 500, errors.New("修改密码失败"))
		return
	}
	s.sessions = map[string]session{}
	s.Store.Event("管理员修改密码，所有会话已失效")
	respond(w, 200, map[string]bool{"ok": true})
}

// 浏览器会省略默认端口，域名大小写也不构成不同的 Origin。
func normalizedOrigin(address string) string {
	u, err := url.Parse(address)
	if err != nil || u.Host == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	port := u.Port()
	if port == "" {
		port = "443"
		if u.Scheme == "http" {
			port = "80"
		}
	}
	return u.Scheme + "://" + net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}
