package server

import (
	"context"
	"os"
	"path/filepath"
	"prismflow/internal/model"
	"testing"
)

func TestIntegrationKernelReplacementRollback(t *testing.T) {
	binary := os.Getenv("MIHOMO_BINARY")
	if binary == "" {
		t.Skip("requires MIHOMO_BINARY")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	s := testServer(t)
	s.Core.SetBinary(binary)
	e := model.Endpoint{Name: "rollback", Protocol: "shadowsocks", Host: "127.0.0.1", Port: 23539, Transport: "tcp", Security: "none", Enabled: true}
	if err = model.InitEndpoint(&e); err != nil {
		t.Fatal(err)
	}
	u := model.NewUser("phone")
	u.EndpointIDs = []string{e.ID}
	if err = s.mutate(func(st *model.State) error {
		st.Endpoints = []model.Endpoint{e}
		st.Users = []model.User{u}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	revision := s.State().Revision
	candidate := filepath.Join(s.Options.DataDir, "invalid-core")
	if err = os.WriteFile(candidate, []byte("invalid binary"), 0755); err != nil {
		t.Fatal(err)
	}
	if err = s.activateCore(context.Background(), candidate); err == nil {
		t.Fatal("invalid core activated")
	}
	if !s.Core.Running() || s.Core.BinaryPath() != binary || s.State().Revision != revision {
		t.Fatal("core replacement failure did not restore previous process and state")
	}
}
