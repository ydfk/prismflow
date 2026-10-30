package server

import (
	"net/http/httptest"
	"prismflow/internal/model"
	"strings"
	"testing"
)

func TestOrdinaryCreationAndAtomicAssignments(t *testing.T) {
	s := testServer(t)
	cookie, csrf := setupLogin(t, s)
	h := s.AdminHandler()
	user := model.NewUser("ordinary-user")
	if err := s.mutate(func(st *model.State) error { st.Users = append(st.Users, user); return nil }); err != nil {
		t.Fatal(err)
	}
	ids := []string{user.ID}
	input := endpointInput{Endpoint: model.Endpoint{Name: "普通 SS", Protocol: "shadowsocks", Transport: "tcp", Security: "none", Host: "127.0.0.1", Enabled: true}, UserIDs: &ids}
	w := call(t, h, "POST", "/api/endpoints", input, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	st := s.State()
	if len(st.Endpoints) != 1 || st.Endpoints[0].Port < 10000 || len(st.Users[0].EndpointIDs) != 1 {
		t.Fatal("ordinary creation did not assign atomically")
	}
	e := st.Endpoints[0]
	input.Endpoint = e
	empty := []string{}
	input.UserIDs = &empty
	w = call(t, h, "PUT", "/api/endpoints/"+e.ID, input, cookie, csrf)
	if w.Code != 200 || len(s.State().Users[0].EndpointIDs) != 0 {
		t.Fatal("edit did not update assignment", w.Body.String())
	}
	input.Name = "bad-user"
	bad := []string{"missing"}
	input.UserIDs = &bad
	before := s.State().Revision
	w = call(t, h, "POST", "/api/endpoints", input, cookie, csrf)
	if w.Code != 400 || s.State().Revision != before || len(s.State().Endpoints) != 1 {
		t.Fatal("invalid assignment partially saved")
	}
}
func TestNativeRequiresKernelAndRedactsPolling(t *testing.T) {
	s := testServer(t)
	cookie, csrf := setupLogin(t, s)
	h := s.AdminHandler()
	e := model.Endpoint{Name: "原生", Protocol: "mixed", Host: "127.0.0.1", Port: 23456, Native: true, NativeConfig: "users: [{username: admin, password: secret-native-password}]", Enabled: false}
	w := call(t, h, "POST", "/api/endpoints", e, cookie, csrf)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "先安装") || len(s.State().Endpoints) != 0 {
		t.Fatal(w.Body.String())
	}
	// 模拟已校验并存储的配置，检查轮询脱敏和显式编辑读取。
	e.ID = "native-id"
	s.mu.Lock()
	s.state.Endpoints = append(s.state.Endpoints, e)
	s.mu.Unlock()
	w = call(t, h, "GET", "/api/state", nil, cookie, csrf)
	if w.Code != 200 || strings.Contains(w.Body.String(), "secret-native-password") {
		t.Fatal("polling leaked native credentials")
	}
	w = call(t, h, "GET", "/api/endpoints/"+e.ID, nil, cookie, csrf)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "secret-native-password") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("explicit edit did not return parameters safely")
	}
	w = call(t, h, "GET", "/api/listener-types", nil, cookie, csrf)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "trusttunnel") {
		t.Fatal("listener catalog missing")
	}
	unauth := httptest.NewRecorder()
	h.ServeHTTP(unauth, httptest.NewRequest("GET", "/api/endpoints/"+e.ID, nil))
	if unauth.Code != 401 {
		t.Fatal("unauthenticated parameter access")
	}
}
