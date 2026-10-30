package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"prismflow/internal/updates"
	"runtime"
	"sync"
	"time"
)

const updateControl = "/var/lib/prismflow-updater"

type panelUpdater struct {
	mu        sync.Mutex
	release   updates.Release
	lastError string
	client    *http.Client
}

func (s *Server) nativeUpdates() bool {
	return s.Options.InstallMode == "native" && runtime.GOOS == "linux"
}
func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request) {
	s.panelUpdates.mu.Lock()
	defer s.panelUpdates.mu.Unlock()
	var operation map[string]any
	if b, err := os.ReadFile(filepath.Join(updateControl, "status.json")); err == nil {
		_ = json.Unmarshal(b, &operation)
	}
	_, autoErr := os.Stat(filepath.Join(updateControl, "requests", "auto-enabled"))
	_, pendingErr := os.Stat(filepath.Join(updateControl, "requests", "request"))
	current := s.Options.Version
	if current == "" {
		current = "dev"
	}
	respond(w, 200, map[string]any{"current": current, "mode": s.Options.InstallMode, "native": s.nativeUpdates(), "auto": autoErr == nil, "pending": pendingErr == nil, "release": s.panelUpdates.release, "operation": operation, "error": s.panelUpdates.lastError})
}
func (s *Server) checkUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.panelUpdates.mu.TryLock() {
		fail(w, 409, errors.New("正在检查版本，请稍后再试"))
		return
	}
	defer s.panelUpdates.mu.Unlock()
	if time.Since(s.panelUpdates.release.CheckedAt) < time.Minute {
		respond(w, 200, s.panelUpdates.release)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	client := s.panelUpdates.client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	release, err := updates.Check(ctx, client, s.Options.Version, updates.ReleasesURL)
	if err != nil {
		s.panelUpdates.lastError = err.Error()
		fail(w, 400, err)
		return
	}
	s.panelUpdates.release = release
	s.panelUpdates.lastError = ""
	respond(w, 200, release)
}
func (s *Server) requestUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.nativeUpdates() {
		fail(w, 400, errors.New("只有通过原生安装脚本部署的 Linux 版本可在面板内更新"))
		return
	}
	// 只提交固定请求标记，下载源、目标路径与特权操作由 root 管理的服务决定。
	path := filepath.Join(updateControl, "requests", "request")
	if err := os.WriteFile(path, []byte("stable\n"), 0600); err != nil {
		fail(w, 400, errors.New("更新服务未安装或请求目录不可写，请重新运行安装脚本"))
		return
	}
	respond(w, 202, map[string]string{"message": "更新请求已提交；更新期间面板和代理连接会短暂中断，请稍后刷新"})
}
func (s *Server) automaticUpdates(w http.ResponseWriter, r *http.Request) {
	if !s.nativeUpdates() {
		fail(w, 400, errors.New("此部署方式不支持面板自动更新"))
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !decode(w, r, &body) {
		return
	}
	path := filepath.Join(updateControl, "requests", "auto-enabled")
	var err error
	if body.Enabled {
		err = os.WriteFile(path, []byte("stable\n"), 0600)
	} else {
		err = os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
	}
	if err != nil {
		fail(w, 400, errors.New("自动更新设置失败，请检查原生更新服务安装状态"))
		return
	}
	respond(w, 200, map[string]bool{"ok": true})
}
