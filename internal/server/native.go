package server

import (
	"context"
	"errors"
	"prismflow/internal/core"
	"prismflow/internal/model"
)

// 原生配置即使暂时停用，也必须经过实际内核校验后再保存。
func (s *Server) validateNativeChanges(ctx context.Context, next model.State) error {
	candidate := model.Clone(next)
	changed := false
	for i, e := range candidate.Endpoints {
		if !e.Native {
			if e.NativeConfig != "" {
				return errors.New("原生参数仅适用于原生监听器模式")
			}
			continue
		}
		same := false
		for _, old := range s.state.Endpoints {
			if old.ID == e.ID && old.Native && old.Protocol == e.Protocol && old.Port == e.Port && old.NativeConfig == e.NativeConfig {
				same = true
			}
		}
		if !same {
			changed = true
			candidate.Endpoints[i].Enabled = true
		}
	}
	if !changed {
		return nil
	}
	if !s.Core.Available() {
		return errors.New("请先安装 Mihomo，再保存原生监听器；原生参数需要实际内核校验")
	}
	b, err := core.Render(candidate, s.Options.Controller, s.Core.Secret, s.Options.WebsiteURL)
	if err != nil {
		return err
	}
	return s.Core.Validate(ctx, b)
}
