package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedPersistence(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Set("test", map[string]string{"secret": "sensitive-marker-123"}); err != nil {
		t.Fatal(err)
	}
	if err = s.DB.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "prismflow.db"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("sensitive-marker-123")) {
		t.Fatal("plaintext secret on disk")
	}
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	var value map[string]string
	if err = s.Get("test", &value); err != nil || value["secret"] != "sensitive-marker-123" {
		t.Fatal("persistence failed", err)
	}
	cipher, err := s.Seal([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	cipher[len(cipher)-1] ^= 1
	if _, err = s.Unseal(cipher); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
}
