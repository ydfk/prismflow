package server

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/scrypt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"prismflow/internal/website"
	"strings"
)

type backupData struct {
	Version  int                        `json:"version"`
	State    model.State                `json:"state"`
	Accounts map[string]json.RawMessage `json:"accounts"`
	Site     []byte                     `json:"site"`
}

func protect(plain []byte, password string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	g, err := backupCipher(password, salt)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	head := append([]byte("PF01"), salt...)
	head = append(head, nonce...)
	return g.Seal(head, nonce, plain, []byte("PF01")), nil
}
func unprotect(b []byte, password string) ([]byte, error) {
	if len(b) < 48 || string(b[:4]) != "PF01" {
		return nil, errors.New("备份格式不正确")
	}
	g, err := backupCipher(password, b[4:20])
	if err != nil {
		return nil, err
	}
	p, err := g.Open(nil, b[20:32], b[32:], []byte("PF01"))
	if err != nil {
		return nil, errors.New("备份密码不正确或文件已损坏")
	}
	return p, nil
}
func backupCipher(password string, salt []byte) (cipher.AEAD, error) {
	if len(password) < 12 || len(password) > 256 {
		return nil, errors.New("备份密码需要 12–256 字节")
	}
	key, err := scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (s *Server) backup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Passphrase string `json:"passphrase"`
	}
	if !decode(w, r, &body) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	payload := backupData{Version: 1, State: s.state, Accounts: map[string]json.RawMessage{}}
	rows, err := s.Store.DB.Query("SELECT key,value FROM kv WHERE key LIKE 'acme-account:%'")
	if err != nil {
		fail(w, 500, err)
		return
	}
	for rows.Next() {
		var k string
		var b []byte
		if err = rows.Scan(&k, &b); err != nil {
			break
		}
		var plain []byte
		plain, err = s.Store.Unseal(b)
		if err != nil {
			break
		}
		payload.Accounts[k] = plain
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(w, 500, errors.New("读取 ACME 账户失败"))
		return
	}
	if s.state.Website.Bundle != "" {
		root := filepath.Join(s.Options.DataDir, "sites", s.state.Website.Bundle)
		var buffer bytes.Buffer
		z := zip.NewWriter(&buffer)
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			rel, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			out, e := z.Create(filepath.ToSlash(rel))
			if e == nil {
				_, e = out.Write(b)
			}
			return e
		})
		closeErr := z.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			fail(w, 500, errors.New("读取网站备份失败"))
			return
		}
		payload.Site = buffer.Bytes()
	}
	plain, err := json.Marshal(payload)
	if err == nil {
		plain, err = protect(plain, body.Passphrase)
	}
	if err != nil {
		fail(w, 400, err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="prismflow.pfb"`)
	_, _ = w.Write(plain)
}
func (s *Server) restore(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
	var body struct {
		Passphrase string `json:"passphrase"`
		Archive    string `json:"archive"`
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&body) != nil || d.Decode(&struct{}{}) != io.EOF {
		fail(w, 400, errors.New("备份请求格式错误"))
		return
	}
	b, err := base64.StdEncoding.DecodeString(body.Archive)
	if err == nil {
		b, err = unprotect(b, body.Passphrase)
	}
	var payload backupData
	if err == nil {
		err = json.Unmarshal(b, &payload)
	}
	if err != nil {
		fail(w, 400, err)
		return
	}
	if payload.Version != 1 {
		fail(w, 400, errors.New("不支持此备份版本"))
		return
	}
	if err = core.Validate(payload.State, s.reserved()); err != nil {
		fail(w, 400, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Core.Running() {
		fail(w, 409, errors.New("恢复前请先停止内核；恢复后在系统设置启动"))
		return
	}
	payload.State.Revision = s.state.Revision + 1
	payload.State.Website.Bundle = ""
	if len(payload.Site) > 0 {
		bundle, e := website.Extract(payload.Site, filepath.Join(s.Options.DataDir, "sites"))
		if e != nil {
			fail(w, 400, e)
			return
		}
		payload.State.Website.Bundle = bundle
	}
	values := map[string]any{"state": payload.State, "applied": model.Default(), "previous-state": model.Default()}
	for k, v := range payload.Accounts {
		if !strings.HasPrefix(k, "acme-account:") {
			fail(w, 400, errors.New("备份包含无效账户记录"))
			return
		}
		values[k] = v
	}
	if err = s.Store.SetMany(values); err != nil {
		fail(w, 500, errors.New("备份恢复失败"))
		return
	}
	s.state = payload.State
	s.applied = model.Default()
	s.Store.Event("已恢复加密备份，等待启动")
	respond(w, 200, map[string]bool{"ok": true})
}
