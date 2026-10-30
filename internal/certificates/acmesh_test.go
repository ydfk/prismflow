package certificates

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestIntegrationAcmeSHBootstrap(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getenv("PRISMFLOW_TEST_ACME") != "1" {
		t.Skip("Linux + PRISMFLOW_TEST_ACME=1 enables official archive bootstrap")
	}
	m := &Manager{Home: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	script, err := m.ensureAcmeSH(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(script) != "acme.sh" {
		t.Fatal("unexpected script")
	}
	out, err := exec.CommandContext(ctx, "sh", script, "--version").Output()
	if err != nil || !strings.Contains(string(out), "v3.1.6") {
		t.Fatalf("official script version check failed: %v", err)
	}
	cached, err := m.ensureAcmeSH(ctx)
	if err != nil || cached != script {
		t.Fatal("cached tool was not reused")
	}
}
