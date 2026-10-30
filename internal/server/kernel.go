package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"runtime"
	"time"
)

func managedCorePath(dir string) string {
	name := "mihomo"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, "bin", name)
}
func (s *Server) coreInstallStatus(w http.ResponseWriter, r *http.Request) {
	s.installMu.Lock()
	job := s.installJob
	s.installMu.Unlock()
	respond(w, 200, map[string]any{"job": job, "path": s.Core.BinaryPath(), "version": model.CoreVersion})
}
func (s *Server) installCore(w http.ResponseWriter, r *http.Request) {
	s.installMu.Lock()
	if s.installJob.Status == "running" {
		s.installMu.Unlock()
		fail(w, 409, errors.New("内核下载正在进行"))
		return
	}
	job := Job{ID: model.ID(), Domain: "Mihomo", Status: "running", Message: "正在从官方发行页下载并校验 SHA-256", CreatedAt: time.Now().UTC()}
	s.installJob = job
	s.installMu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(s.ctx, 6*time.Minute)
		defer cancel()
		candidate := managedCorePath(s.Options.DataDir) + ".candidate"
		err := core.Download(ctx, candidate)
		if err == nil {
			err = s.activateCore(ctx, candidate)
		}
		_ = os.Remove(candidate)
		job.Status = "done"
		job.Message = "内核已安装，现有配置已自动应用"
		if err != nil {
			job.Status = "failed"
			job.Message = "内核安装失败，原内核和配置已保留：" + err.Error()
		}
		s.installMu.Lock()
		s.installJob = job
		s.installMu.Unlock()
		s.Store.Event("内核安装任务：" + job.Status)
	}()
	respond(w, 202, job)
}
func (s *Server) activateCore(ctx context.Context, candidate string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := managedCorePath(s.Options.DataDir)
	backup := path + ".previous"
	oldPath := s.Core.BinaryPath()
	wasRunning := s.Core.Running()
	existed := false
	s.Core.Stop()
	if _, err := os.Stat(path); err == nil {
		existed = true
		_ = os.Remove(backup)
		if err = os.Rename(path, backup); err != nil {
			if wasRunning {
				s.restartAppliedCore()
			}
			return err
		}
	}
	err := os.Rename(candidate, path)
	if err == nil {
		s.Core.SetBinary(path)
		err = s.saveLocked(ctx, model.Clone(s.state))
	}
	if err == nil {
		return nil
	}
	s.Core.Stop()
	_ = os.Remove(path)
	if existed {
		if restoreErr := os.Rename(backup, path); restoreErr != nil {
			return errors.New("内核替换失败，旧文件保存在 .previous，请检查权限")
		}
	}
	s.Core.SetBinary(oldPath)
	if wasRunning {
		b, e := core.Render(s.applied, s.Options.Controller, s.Core.Secret, s.Options.WebsiteURL)
		if e == nil {
			_ = s.Core.Publish(context.Background(), b, func() error { return nil })
		}
	}
	return err
}

func (s *Server) restartAppliedCore() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	b, err := core.Render(s.applied, s.Options.Controller, s.Core.Secret, s.Options.WebsiteURL)
	if err == nil {
		_ = s.Core.Publish(ctx, b, func() error { return nil })
	}
}
