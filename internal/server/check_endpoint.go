package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"strconv"
	"time"
)

type checkStep struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}
type endpointCheck struct {
	State     string      `json:"state"`
	CheckedAt time.Time   `json:"checkedAt"`
	DelayMS   int64       `json:"delayMs"`
	Steps     []checkStep `json:"steps"`
	Message   string      `json:"message"`
}

func (s *Server) checkEndpoint(w http.ResponseWriter, r *http.Request) {
	if !s.checkMu.TryLock() {
		fail(w, 409, errors.New("已有检测正在进行，请稍后再试"))
		return
	}
	defer s.checkMu.Unlock()
	result := endpointCheck{State: "unknown", CheckedAt: time.Now(), Steps: []checkStep{}}
	finish := func(message string) { result.Message = message; respond(w, 200, result) }
	st := s.Applied()
	saved := s.State()
	var endpoint *model.Endpoint
	for i, e := range saved.Endpoints {
		if e.ID == r.PathValue("id") {
			endpoint = &saved.Endpoints[i]
		}
	}
	if endpoint == nil {
		fail(w, 404, errors.New("入口不存在"))
		return
	}
	if !endpoint.Enabled {
		finish("入口已停用")
		return
	}
	if !s.Core.Running() {
		result.State = "unavailable"
		finish("内核未运行")
		return
	}
	instance := s.Core.InstanceID()
	if st.Revision != saved.Revision {
		finish("配置尚未生效，请先检查内核")
		return
	}
	result.Steps = append(result.Steps, checkStep{"配置与内核", true, "使用当前已生效的配置"})
	if endpoint.Native {
		finish("原生监听器自行管理客户端参数，暂不自动执行真实协议检测")
		return
	}
	var user *model.User
	for i, u := range st.Users {
		if model.Assigned(u, *endpoint) {
			user = &st.Users[i]
			break
		}
	}
	if user == nil {
		finish("没有启用且已分配的用户，入口尚未监听")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 18*time.Second)
	defer cancel()
	// TCP 端口只是中间步骤，最终结果仍取决于真实协议请求。
	if endpoint.Protocol != "hysteria2" && endpoint.Protocol != "tuic" {
		connection, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(endpoint.Host, strconv.Itoa(endpoint.ClientPort())))
		result.Steps = append(result.Steps, checkStep{"入口连接", err == nil, "检查订阅中的服务器地址与端口"})
		if err != nil {
			result.State = "unavailable"
			finish("无法连接入口地址；检查 DNS、端口映射或防火墙")
			return
		}
		connection.Close()
	}
	result.Steps = append(result.Steps, checkStep{"订阅参数", true, "保留实际地址、SNI、证书校验和所选用户凭据"})
	base := filepath.Join(s.Options.DataDir, "checks")
	if err := os.MkdirAll(base, 0700); err != nil {
		finish("无法创建检测目录")
		return
	}
	delay, err := core.CheckProxy(ctx, s.Core.BinaryPath(), base, core.Proxy(*endpoint, *user), []string{"https://www.gstatic.com/generate_204", "https://cp.cloudflare.com/generate_204"})
	if s.Applied().Revision != st.Revision || s.Core.InstanceID() != instance {
		finish("检测期间配置或内核发生变化，请重新检测")
		return
	}
	result.Steps = append(result.Steps, checkStep{"真实协议请求", err == nil, "通过临时客户端请求 204 检测地址；没有直连回退"})
	if err != nil {
		result.State = "unavailable"
		finish(err.Error())
		return
	}
	result.State = "available"
	result.DelayMS = delay
	finish("服务器侧真实连接通过；外部客户端的公网可达性仍需在客户端验证")
}
