package server

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"strings"
	"time"
)

func readCertificateFile(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("证书文件必须使用绝对路径")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", errors.New("无法读取证书文件，请检查路径与服务账户权限")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return "", errors.New("证书文件须为小于 1 MiB 的普通文件")
	}
	b, err := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if err != nil || len(b) > 1<<20 {
		return "", errors.New("读取证书文件失败")
	}
	return string(b), nil
}
func (s *Server) loadCertificateFiles(st *model.State) error {
	for i := range st.Certificates {
		c := &st.Certificates[i]
		if c.Method != "path" {
			continue
		}
		pem, err := readCertificateFile(c.CertPath)
		if err != nil {
			return err
		}
		key, err := readCertificateFile(c.KeyPath)
		if err != nil {
			return err
		}
		cert, err := core.ParseCertificate(pem, key)
		if err != nil {
			return err
		}
		if err = cert.VerifyHostname(c.Domain); err != nil {
			return errors.New("证书域名不匹配")
		}
		c.PEM = pem
		c.Key = key
		c.ExpiresAt = cert.NotAfter
		c.LastError = ""
	}
	return nil
}
func (s *Server) linkCertificatePath(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID       string `json:"id"`
		Domain   string `json:"domain"`
		CertPath string `json:"certPath"`
		KeyPath  string `json:"keyPath"`
	}
	if !decode(w, r, &in) {
		return
	}
	mutationResult(w, s.mutate(func(st *model.State) error {
		c := model.Certificate{ID: in.ID, Domain: in.Domain, Method: "path", CertPath: in.CertPath, KeyPath: in.KeyPath}
		if in.ID != "" {
			for i := range st.Certificates {
				if st.Certificates[i].ID == in.ID {
					st.Certificates[i] = c
					return nil
				}
			}
			return errors.New("证书不存在")
		}
		if len(st.Certificates) >= 30 {
			return errors.New("最多保存 30 张证书")
		}
		c.ID = model.ID()
		st.Certificates = append(st.Certificates, c)
		return nil
	}))
}
func (s *Server) certificateFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !filepath.IsAbs(path) {
		fail(w, 400, errors.New("请输入服务器上的绝对目录"))
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		fail(w, 400, errors.New("目录不存在或服务账户没有读取权限"))
		return
	}
	type entry struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		Directory bool   `json:"directory"`
	}
	result := []entry{}
	for _, e := range entries {
		full := filepath.Join(path, e.Name())
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if info.IsDir() || ext == ".pem" || ext == ".crt" || ext == ".cer" || ext == ".key" {
			result = append(result, entry{e.Name(), full, info.IsDir()})
		}
		if len(result) >= 200 {
			break
		}
	}
	respond(w, 200, map[string]any{"path": filepath.Clean(path), "parent": filepath.Dir(path), "entries": result})
}
func (s *Server) refreshSystemCertificateFiles() {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := model.Clone(s.state)
	err := s.loadCertificateFiles(&next)
	if err != nil {
		changed := false
		for i, c := range s.state.Certificates {
			if c.Method == "path" && c.LastError != err.Error() {
				next.Certificates[i] = c
				next.Certificates[i].LastError = err.Error()
				changed = true
			}
		}
		if changed {
			if s.Store.Set("state", next) == nil {
				s.state = next
			}
			s.Store.Event("系统证书暂不可用，已保留原运行证书")
		}
		return
	}
	changed := false
	for i, c := range next.Certificates {
		if c.Method == "path" && (c.PEM != s.state.Certificates[i].PEM || c.Key != s.state.Certificates[i].Key || s.state.Certificates[i].LastError != "") {
			changed = true
		}
	}
	if changed {
		if err = s.saveLocked(s.ctx, next); err != nil {
			s.Store.Event("系统证书更新未应用，保留运行配置")
		} else {
			s.Store.Event("系统证书文件已自动重新加载")
		}
	}
}
func (s *Server) watchCertificateFiles() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.refreshSystemCertificateFiles()
		}
	}
}
