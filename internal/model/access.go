package model

// 公网地址与实际监听地址分开保存；空字段兼容已有数据。
type AccessSettings struct {
	BaseDomain string `json:"baseDomain"`
	PanelURL   string `json:"panelUrl"`
	XHTTPTLS   string `json:"xhttpTls"`
}

func (e Endpoint) ExternalTLS() bool { return e.TLSMode == "external" }
func (e Endpoint) ClientPort() int {
	if e.ExternalTLS() {
		if e.PublicPort != 0 {
			return e.PublicPort
		}
		return 443
	}
	return e.Port
}
func (e Endpoint) ListenHost() string {
	if e.ExternalTLS() {
		return "127.0.0.1"
	}
	return "0.0.0.0"
}
