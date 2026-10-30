package core

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"prismflow/internal/model"

	"gopkg.in/yaml.v3"
)

// 临时客户端复用真实订阅参数，不改写服务器地址、认证、TLS 或线路；无直连回退。
func CheckProxy(ctx context.Context, binary, base string, proxy map[string]any, targets []string) (int64, error) {
	dir, err := os.MkdirTemp(base, "probe-")
	if err != nil {
		return 0, fmt.Errorf("无法创建检测工作目录")
	}
	defer os.RemoveAll(dir)
	socket, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	address := socket.Addr().String()
	socket.Close()
	manager := NewManager(binary, dir, address, model.Token(24))
	defer manager.Stop()
	proxy["name"] = "prismflow-check"
	b, err := yaml.Marshal(map[string]any{"mode": "rule", "log-level": "silent", "ipv6": true, "external-controller": address, "secret": manager.Secret, "listeners": []any{}, "proxies": []any{proxy}, "rules": []string{"MATCH,prismflow-check"}, "profile": map[string]any{"store-selected": false}})
	if err != nil {
		return 0, err
	}
	if err = manager.Publish(ctx, b, func() error { return nil }); err != nil {
		return 0, fmt.Errorf("临时客户端启动或参数校验失败")
	}
	for _, target := range targets {
		query := url.Values{"url": {target}, "timeout": {"2000"}, "expected": {"204"}}
		var response struct {
			Delay *int64 `json:"delay"`
		}
		if manager.Request(ctx, "/proxies/prismflow-check/delay?"+query.Encode(), &response) == nil && response.Delay != nil {
			return *response.Delay, nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	return 0, fmt.Errorf("节点真实请求未通过；请检查公网地址、端口、防火墙、证书和用户凭据")
}
