package server

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"prismflow/internal/website"
	"time"
)

func (s *Server) coreAction(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	action := r.PathValue("action")
	if action == "stop" {
		s.Core.Stop()
		s.Store.Event("管理员停止内核")
		respond(w, 200, map[string]bool{"ok": true})
		return
	}
	target := model.Clone(s.state)
	if action == "rollback" {
		if err := s.Store.Get("previous-state", &target); err != nil {
			fail(w, 409, errors.New("没有可恢复的历史配置"))
			return
		}
		target.Revision = s.state.Revision + 1
	} else if action != "apply" && action != "validate" {
		http.NotFound(w, r)
		return
	}
	if err := s.loadCertificateFiles(&target); err != nil {
		fail(w, 400, err)
		return
	}
	if err := core.ValidateReady(target, s.reserved()); err != nil {
		fail(w, 400, err)
		return
	}
	b, err := core.Render(target, s.Options.Controller, s.Core.Secret, s.Options.WebsiteURL)
	if err != nil {
		fail(w, 400, err)
		return
	}
	if action == "validate" {
		mutationResult(w, s.Core.Validate(r.Context(), b))
		return
	}
	previous := s.applied
	err = s.Core.Publish(r.Context(), b, func() error {
		// 使用同一个 SQLite 事务保证应用快照、回滚快照和草稿的一致性。
		return s.commitPublish(previous, target)
	})
	if err != nil {
		s.Store.Event("配置应用失败；请检查端口占用、证书及内核版本")
		fail(w, 400, err)
		return
	}
	s.applied = target
	if action == "rollback" {
		s.state = target
	}
	s.Store.Event("配置已通过内核校验并应用")
	respond(w, 200, map[string]bool{"ok": true})
}
func (s *Server) commitPublish(previous, target model.State) error {
	values := map[string]any{"applied": target, "previous-state": previous}
	if target.Revision != s.state.Revision {
		values["state"] = target
	}
	return s.Store.SetMany(values)
}
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	result, err := s.Store.Events()
	if err != nil {
		fail(w, 500, errors.New("读取事件失败"))
		return
	}
	respond(w, 200, result)
}
func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	hours := 24
	switch r.URL.Query().Get("range") {
	case "1h":
		hours = 1
	case "7d":
		hours = 168
	case "30d":
		hours = 720
	}
	points, err := s.Metrics.History(time.Now().Add(-time.Duration(hours) * time.Hour).Unix())
	if err != nil {
		fail(w, 500, errors.New("读取流量历史失败"))
		return
	}
	respond(w, 200, points)
}
func (s *Server) updateWebsite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Template    string `json:"template"`
		UseTemplate bool   `json:"useTemplate"`
		Disabled    *bool  `json:"disabled"`
	}
	if !decode(w, r, &body) {
		return
	}
	mutationResult(w, s.mutate(func(st *model.State) error {
		if body.Disabled != nil {
			st.Website.Disabled = *body.Disabled
		}
		st.Website.Title = body.Title
		st.Website.Description = body.Description
		st.Website.Template = body.Template
		if body.UseTemplate {
			st.Website.Bundle = ""
		}
		return nil
	}))
}
func (s *Server) uploadWebsite(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	b, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, 400, errors.New("ZIP 文件不能超过 10 MiB"))
		return
	}
	bundle, err := website.Extract(b, filepath.Join(s.Options.DataDir, "sites"))
	if err == nil {
		err = s.mutate(func(st *model.State) error { st.Website.Bundle = bundle; return nil })
	}
	mutationResult(w, err)
}
