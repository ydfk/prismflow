package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"prismflow/internal/core"
	"prismflow/internal/model"
	"strings"
	"testing"
)

func TestSubscriptionHeaderPrefixAndWebsiteSwitch(t *testing.T) {
	s := testServer(t)
	cookie, csrf := setupLogin(t, s)
	h := s.AdminHandler()
	user := model.NewUser("Alice")
	endpoint := model.Endpoint{Name: "SS", Protocol: "shadowsocks", Transport: "tcp", Security: "none", Host: "localhost", Port: 23456, Enabled: true}
	if err := model.InitEndpoint(&endpoint); err != nil {
		t.Fatal(err)
	}
	user.EndpointIDs = []string{endpoint.ID}
	st := model.Default()
	st.Endpoints = []model.Endpoint{endpoint}
	st.Users = []model.User{user}
	st.Traffic = model.TrafficSettings{MaxBytes: 1073741824, ResetDay: 20}
	s.mu.Lock()
	s.state = st
	s.applied = model.Clone(st)
	s.mu.Unlock()
	for _, format := range []string{"", "?format=links"} {
		w := httptest.NewRecorder()
		s.PublicHandler().ServeHTTP(w, httptest.NewRequest("GET", "/s/"+user.Token+format, nil))
		if w.Code != 200 || !strings.Contains(w.Header().Get("Subscription-Userinfo"), "total=1073741824") || w.Header().Get("X-PrismFlow-Traffic-Reset-At") == "" {
			t.Fatal(w.Code, w.Header())
		}
		if format == "?format=links" {
			if w.Header().Get("Content-Type") != "text/plain; charset=utf-8" || w.Header().Get("Content-Disposition") != "" {
				t.Fatal("通用订阅应直接返回纯文本，不应触发 YAML 下载", w.Header())
			}
			decoded, err := base64.StdEncoding.DecodeString(w.Body.String())
			if err != nil {
				t.Fatalf("通用订阅不是有效的 Base64：%v", err)
			}
			if string(decoded) != core.Links(st, user) || !strings.HasPrefix(string(decoded), "ss://") {
				t.Fatal("通用订阅解码后应为用户的节点分享链接")
			}
		} else if w.Header().Get("Content-Type") != "text/yaml; charset=utf-8" || !strings.Contains(w.Header().Get("Content-Disposition"), "prismflow.yaml") || !strings.Contains(w.Body.String(), "Alice · SS") {
			t.Fatal("Mihomo 订阅应保留 YAML 配置和下载文件名")
		}
	}
	yaml, _ := core.Subscription(st, user)
	if !strings.Contains(string(yaml), "Alice · SS") {
		t.Fatal("missing prefix")
	}
	body := map[string]any{"title": st.Website.Title, "description": st.Website.Description, "template": st.Website.Template, "disabled": true}
	w := call(t, h, "PUT", "/api/website", body, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	s.PublicHandler().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 404 {
		t.Fatal("disabled website still visible")
	}
	w = httptest.NewRecorder()
	s.PublicHandler().ServeHTTP(w, httptest.NewRequest("GET", "/s/"+user.Token, nil))
	if w.Code != 200 {
		t.Fatal("disabled website broke subscription")
	}
	// 原生监听器不应假装已经通过真实协议检测。
	w = call(t, h, "POST", "/api/endpoints/"+endpoint.ID+"/check", nil, cookie, csrf)
	var result endpointCheck
	json.Unmarshal(w.Body.Bytes(), &result)
	if w.Code != 200 || result.State == "available" {
		t.Fatal(w.Body.String())
	}
}
