package core

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"prismflow/internal/model"
	"strings"
)

// 原生参数只属于一个监听器，不能覆盖面板管理的名称、类型和端口。
func NativeListener(e model.Endpoint) (map[string]any, error) {
	if !model.NativeSupported(e.Protocol) {
		return nil, errors.New("内核不支持此监听器类型")
	}
	if len(e.NativeConfig) > 65536 {
		return nil, errors.New("原生参数不能超过 64 KiB")
	}
	options := map[string]any{}
	if strings.TrimSpace(e.NativeConfig) != "" {
		decoder := yaml.NewDecoder(strings.NewReader(e.NativeConfig))
		if err := decoder.Decode(&options); err != nil {
			return nil, errors.New("原生参数须为 YAML 或 JSON 对象")
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return nil, errors.New("原生参数只能包含一个对象")
		}
	}
	if options == nil {
		return nil, errors.New("原生参数须为对象")
	}
	for _, key := range []string{"name", "type", "port"} {
		if _, exists := options[key]; exists {
			return nil, fmt.Errorf("请在表单中设置 %s，原生参数不能覆盖它", key)
		}
	}
	if _, exists := options["listen"]; !exists {
		options["listen"] = "127.0.0.1"
	}
	options["name"] = "pf-" + e.ID
	options["type"] = e.Protocol
	if e.Protocol != "tun" {
		options["port"] = e.Port
	}
	// Tunnel 启动日志使用目标字段；约束类型以便准确跟踪就绪状态。
	if e.Protocol == "tunnel" {
		target, ok := options["target"].(string)
		if !ok || strings.TrimSpace(target) == "" {
			return nil, errors.New("Tunnel 需要 target 目标地址")
		}
	}
	return options, nil
}
