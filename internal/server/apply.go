package server

import (
	"bytes"
	"context"
	"prismflow/internal/core"
	"prismflow/internal/model"
)

// 保存和生效在同一事务边界完成；校验或启动失败时保留原配置。
func (s *Server) saveLocked(ctx context.Context, next model.State) error {
	if err := s.loadCertificateFiles(&next); err != nil {
		return err
	}
	if err := core.Validate(next, s.reserved()); err != nil {
		return err
	}
	if err := s.validateNativeChanges(ctx, next); err != nil {
		return err
	}
	next.Revision = s.state.Revision + 1
	if !s.Core.Available() {
		if err := s.Store.Set("state", next); err != nil {
			return err
		}
		s.state = next
		return nil
	}
	b, err := core.Render(next, s.Options.Controller, s.Core.Secret, s.Options.WebsiteURL)
	if err != nil {
		return err
	}
	old, err := core.Render(s.applied, s.Options.Controller, s.Core.Secret, s.Options.WebsiteURL)
	if err != nil {
		return err
	}
	previous := model.Clone(s.applied)
	commit := func() error {
		return s.Store.SetMany(map[string]any{"state": next, "applied": next, "previous-state": previous})
	}
	if !bytes.Equal(b, old) || (!s.Core.Running() && hasAssignedUser(next)) {
		if err = core.ValidateReady(next, s.reserved()); err != nil {
			return err
		}
		if err = s.Core.Publish(ctx, b, commit); err != nil {
			return err
		}
	} else if err = commit(); err != nil {
		return err
	}
	s.state = next
	s.applied = next
	return nil
}
func hasAssignedUser(st model.State) bool {
	for _, e := range st.Endpoints {
		if e.Native && e.Enabled {
			return true
		}
		for _, u := range st.Users {
			if model.Assigned(u, e) {
				return true
			}
		}
	}
	return false
}
