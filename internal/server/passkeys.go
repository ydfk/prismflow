package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/bcrypt"

	"prismflow/internal/model"
)

const (
	passkeyTimeout = 2 * time.Minute
	maxPasskeys    = 16
)

type passkeyInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	RPID       string     `json:"rpId"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

type storedPasskey struct {
	passkeyInfo
	Credential webauthn.Credential `json:"credential"`
}

type passkeyUser struct {
	adminAccount
	rpID string
}

func (u passkeyUser) WebAuthnID() []byte          { return u.PasskeyID }
func (u passkeyUser) WebAuthnName() string        { return u.Username }
func (u passkeyUser) WebAuthnDisplayName() string { return u.Username }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential {
	keys := make([]webauthn.Credential, 0, len(u.Passkeys))
	for _, key := range u.Passkeys {
		if key.RPID == u.rpID {
			keys = append(keys, key.Credential)
		}
	}
	return keys
}

type passkeyFlow struct {
	session      webauthn.SessionData
	origin       string
	adminSession string
	name         string
}

type passkeyResponse struct {
	FlowID     string          `json:"flowId"`
	Credential json.RawMessage `json:"credential"`
}

func (s *Server) passkeyRP(w http.ResponseWriter, r *http.Request) (*webauthn.WebAuthn, string) {
	origin := r.Header.Get("Origin")
	if normalizedOrigin(origin) == "" || !s.sameOrigin(r) {
		fail(w, 403, errors.New("请从面板访问地址使用 Passkey"))
		return nil, ""
	}
	u, _ := url.Parse(origin)
	host := strings.ToLower(u.Hostname())
	if net.ParseIP(host) != nil || (u.Scheme != "https" && host != "localhost") {
		fail(w, 400, errors.New("Passkey 需要 HTTPS 域名，或通过 localhost 访问"))
		return nil, ""
	}
	timeout := webauthn.TimeoutConfig{Enforce: true, Timeout: passkeyTimeout, TimeoutUVD: passkeyTimeout}
	rp, err := webauthn.New(&webauthn.Config{
		RPID: host, RPDisplayName: "PrismFlow", RPOrigins: []string{origin},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification:   protocol.VerificationRequired,
		},
		Timeouts: webauthn.TimeoutsConfig{Login: timeout, Registration: timeout},
	})
	if err != nil {
		fail(w, 400, errors.New("当前面板地址无法使用 Passkey"))
		return nil, ""
	}
	return rp, origin
}

// 挑战只使用一次，并与当前来源及登记时的管理员会话绑定。调用方持有 authMu。
func (s *Server) startPasskeyFlow(w http.ResponseWriter, flow passkeyFlow, options any) {
	now := time.Now()
	for id, pending := range s.passkeyFlows {
		if !now.Before(pending.session.Expires) {
			delete(s.passkeyFlows, id)
		}
	}
	if len(s.passkeyFlows) >= 1024 {
		fail(w, 429, errors.New("当前 Passkey 请求过多，请稍后重试"))
		return
	}
	if s.passkeyFlows == nil {
		s.passkeyFlows = make(map[string]passkeyFlow)
	}
	id := model.Token(24)
	s.passkeyFlows[id] = flow
	respond(w, 200, map[string]any{"flowId": id, "options": options})
}

func (s *Server) takePasskeyFlow(w http.ResponseWriter, id, origin, adminSession string) (passkeyFlow, bool) {
	flow, exists := s.passkeyFlows[id]
	if !exists || flow.origin != origin || flow.adminSession != adminSession || !time.Now().Before(flow.session.Expires) {
		fail(w, 400, errors.New("Passkey 请求无效或已过期，请重新发起"))
		return passkeyFlow{}, false
	}
	delete(s.passkeyFlows, id)
	return flow, true
}

func (s *Server) passkeyAdminSession(r *http.Request) (string, bool) {
	cookie, err := r.Cookie("prismflow_session")
	if err != nil {
		return "", false
	}
	key := tokenHash(cookie.Value)
	sess, ok := s.sessions[key]
	return key, ok && time.Now().Before(sess.Expires)
}

func (s *Server) listPasskeys(w http.ResponseWriter, r *http.Request) {
	var admin adminAccount
	if s.Store.Get("admin", &admin) != nil {
		fail(w, 500, errors.New("无法读取 Passkey"))
		return
	}
	keys := make([]passkeyInfo, 0, len(admin.Passkeys))
	for _, key := range admin.Passkeys {
		keys = append(keys, key.passkeyInfo)
	}
	respond(w, 200, keys)
}

func (s *Server) beginPasskeyRegistration(w http.ResponseWriter, r *http.Request) {
	rp, origin := s.passkeyRP(w, r)
	if rp == nil {
		return
	}
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		fail(w, 400, errors.New("Passkey 名称须为 1–64 个字符"))
		return
	}
	s.authMu.Lock()
	defer s.authMu.Unlock()
	adminSession, ok := s.passkeyAdminSession(r)
	if !ok {
		fail(w, 401, errors.New("会话已过期，请重新登录"))
		return
	}
	var admin adminAccount
	if s.Store.Get("admin", &admin) != nil {
		fail(w, 500, errors.New("无法读取管理员"))
		return
	}
	attempt := attemptKey(r, "enroll-passkey")
	if !s.allowAttempt(w, attempt, time.Now()) {
		return
	}
	if !validPassword(body.Password) || bcrypt.CompareHashAndPassword([]byte(admin.Hash), []byte(body.Password)) != nil {
		fail(w, 403, errors.New("当前密码不正确"))
		return
	}
	delete(s.attempts, attempt)
	if len(admin.Passkeys) >= maxPasskeys {
		fail(w, 409, errors.New("Passkey 已达 16 个，请先撤销不用的凭据"))
		return
	}
	if len(admin.PasskeyID) == 0 {
		admin.PasskeyID = []byte(model.Token(32))
		if s.Store.Set("admin", admin) != nil {
			fail(w, 500, errors.New("无法保存管理员标识"))
			return
		}
	}
	user := passkeyUser{admin, rp.Config.RPID}
	excluded := make([]protocol.CredentialDescriptor, 0, len(admin.Passkeys))
	for _, credential := range user.WebAuthnCredentials() {
		excluded = append(excluded, credential.Descriptor())
	}
	options, session, err := rp.BeginRegistration(user, webauthn.WithExclusions(excluded))
	if err != nil {
		fail(w, 500, errors.New("无法发起 Passkey 登记"))
		return
	}
	s.startPasskeyFlow(w, passkeyFlow{session: *session, origin: origin, adminSession: adminSession, name: name}, options)
}

func (s *Server) finishPasskeyRegistration(w http.ResponseWriter, r *http.Request) {
	rp, origin := s.passkeyRP(w, r)
	if rp == nil {
		return
	}
	var body passkeyResponse
	if !decode(w, r, &body) {
		return
	}
	s.authMu.Lock()
	defer s.authMu.Unlock()
	adminSession, ok := s.passkeyAdminSession(r)
	if !ok {
		fail(w, 401, errors.New("会话已过期，请重新登录"))
		return
	}
	flow, ok := s.takePasskeyFlow(w, body.FlowID, origin, adminSession)
	if !ok {
		return
	}
	var admin adminAccount
	if s.Store.Get("admin", &admin) != nil {
		fail(w, 500, errors.New("无法读取管理员"))
		return
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(body.Credential)
	if err != nil {
		fail(w, 400, errors.New("Passkey 登记响应无效"))
		return
	}
	credential, err := rp.CreateCredential(passkeyUser{admin, rp.Config.RPID}, flow.session, parsed)
	if err != nil {
		fail(w, 400, errors.New("Passkey 登记验证失败，请重试"))
		return
	}
	id := base64.RawURLEncoding.EncodeToString(credential.ID)
	if len(admin.Passkeys) >= maxPasskeys || passkeyIndex(admin, id) >= 0 {
		fail(w, 409, errors.New("Passkey 已登记或已达数量上限"))
		return
	}
	info := passkeyInfo{ID: id, Name: flow.name, RPID: rp.Config.RPID, CreatedAt: time.Now().UTC()}
	admin.Passkeys = append(admin.Passkeys, storedPasskey{info, *credential})
	if s.Store.Set("admin", admin) != nil {
		fail(w, 500, errors.New("无法保存 Passkey"))
		return
	}
	s.Store.Event("管理员登记 Passkey")
	respond(w, 201, info)
}

func (s *Server) beginPasskeyLogin(w http.ResponseWriter, r *http.Request) {
	rp, origin := s.passkeyRP(w, r)
	if rp == nil {
		return
	}
	s.authMu.Lock()
	defer s.authMu.Unlock()
	var admin adminAccount
	if s.Store.Get("admin", &admin) != nil || len((passkeyUser{admin, rp.Config.RPID}).WebAuthnCredentials()) == 0 {
		fail(w, 409, errors.New("当前域名没有可用的 Passkey，请先用密码登录并登记"))
		return
	}
	options, session, err := rp.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		fail(w, 500, errors.New("无法发起 Passkey 登录"))
		return
	}
	s.startPasskeyFlow(w, passkeyFlow{session: *session, origin: origin}, options)
}

func (s *Server) finishPasskeyLogin(w http.ResponseWriter, r *http.Request) {
	rp, origin := s.passkeyRP(w, r)
	if rp == nil {
		return
	}
	var body passkeyResponse
	if !decode(w, r, &body) {
		return
	}
	s.authMu.Lock()
	defer s.authMu.Unlock()
	flow, ok := s.takePasskeyFlow(w, body.FlowID, origin, "")
	if !ok {
		return
	}
	now := time.Now().UTC()
	attempt := attemptKey(r, "passkey")
	if !s.allowAttempt(w, attempt, now) {
		return
	}
	var admin adminAccount
	if s.Store.Get("admin", &admin) != nil {
		fail(w, 500, errors.New("无法读取管理员"))
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(body.Credential)
	if err != nil {
		fail(w, 400, errors.New("Passkey 登录响应无效"))
		return
	}
	_, credential, err := rp.ValidatePasskeyLogin(func(_, handle []byte) (webauthn.User, error) {
		if !bytes.Equal(handle, admin.PasskeyID) {
			return nil, errors.New("Passkey 不属于管理员")
		}
		return passkeyUser{admin, rp.Config.RPID}, nil
	}, flow.session, parsed)
	if err != nil || credential.Authenticator.CloneWarning {
		fail(w, 401, errors.New("Passkey 登录验证失败，请重试或使用密码登录"))
		return
	}
	index := passkeyIndex(admin, base64.RawURLEncoding.EncodeToString(credential.ID))
	admin.Passkeys[index].Credential = *credential
	admin.Passkeys[index].LastUsedAt = &now
	if s.Store.Set("admin", admin) != nil {
		fail(w, 500, errors.New("无法更新 Passkey"))
		return
	}
	delete(s.attempts, attempt)
	s.issueSession(w, now, strings.HasPrefix(origin, "https://") || s.secureCookie(r))
}

func passkeyIndex(admin adminAccount, id string) int {
	return slices.IndexFunc(admin.Passkeys, func(key storedPasskey) bool { return key.ID == id })
}

func (s *Server) deletePasskey(w http.ResponseWriter, r *http.Request) {
	s.authMu.Lock()
	defer s.authMu.Unlock()
	if _, ok := s.passkeyAdminSession(r); !ok {
		fail(w, 401, errors.New("会话已过期，请重新登录"))
		return
	}
	var admin adminAccount
	if s.Store.Get("admin", &admin) != nil {
		fail(w, 500, errors.New("无法读取 Passkey"))
		return
	}
	index := passkeyIndex(admin, r.PathValue("id"))
	if index < 0 {
		fail(w, 404, errors.New("Passkey 不存在"))
		return
	}
	admin.Passkeys = slices.Delete(admin.Passkeys, index, index+1)
	if s.Store.Set("admin", admin) != nil {
		fail(w, 500, errors.New("无法撤销 Passkey"))
		return
	}
	s.Store.Event("管理员撤销 Passkey")
	respond(w, 200, map[string]bool{"ok": true})
}
