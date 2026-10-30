package website

import (
	"archive/zip"
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"prismflow/internal/model"
	"strings"
	"testing"
)

func zipFile(t *testing.T, name, body string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte(body))
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestArchiveBoundaries(t *testing.T) {
	for _, name := range []string{"../escape.html", "/escape.html", "a\\escape.html", "C:escape.html", "app.js"} {
		t.Run(name, func(t *testing.T) {
			if _, err := Extract(zipFile(t, name, "x"), t.TempDir()); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
	if _, err := Extract(zipFile(t, "nested/index.html", "x"), t.TempDir()); err == nil {
		t.Fatal("missing root index accepted")
	}
}
func TestServeUploadedAndEscapedTemplate(t *testing.T) {
	dir := t.TempDir()
	id, err := Extract(zipFile(t, "index.html", "<h1>Own site</h1>"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, id, "index.html")); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://example.com/", nil)
	w := httptest.NewRecorder()
	Serve(w, r, model.Website{Bundle: id}, dir)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Own site") {
		t.Fatalf("uploaded site failed: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	Serve(w, r, model.Website{Title: "<script>alert(1)</script>", Template: "journal"}, dir)
	if strings.Contains(w.Body.String(), "<script>") {
		t.Fatal("template injection")
	}
}
