package core

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"prismflow/internal/model"
	"testing"
	"time"
)

func TestIntegrationIsolatedProtocolCheck(t *testing.T) {
	binary := integrationBinary(t)
	st := testState(t)
	endpoint := st.Endpoints[5]
	endpoint.Port = freePort(t)
	st.Endpoints = []model.Endpoint{endpoint}
	st.Users[0].EndpointIDs = []string{endpoint.ID}
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer destination.Close()
	serving := NewManager(binary, t.TempDir(), fmt.Sprintf("127.0.0.1:%d", freePort(t)), model.Token(24))
	defer serving.Stop()
	rendered, err := Render(st, serving.Address, serving.Secret, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = serving.Publish(context.Background(), rendered, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	instance := serving.InstanceID()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err = CheckProxy(ctx, binary, t.TempDir(), Proxy(endpoint, st.Users[0]), []string{destination.URL}); err != nil {
		t.Fatal(err)
	}
	bad := Proxy(endpoint, st.Users[0])
	bad["password"] = "invalid-password"
	if _, err = CheckProxy(ctx, binary, t.TempDir(), bad, []string{destination.URL}); err == nil {
		t.Fatal("invalid credentials passed or used direct fallback")
	}
	if serving.InstanceID() != instance || !serving.Running() {
		t.Fatal("check disturbed serving core")
	}
}
