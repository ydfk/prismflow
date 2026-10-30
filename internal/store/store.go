package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"prismflow/internal/model"
)

type Store struct {
	DB  *sql.DB
	key []byte
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "master.key")
	key, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err == nil {
			err = os.WriteFile(path, key, 0600)
		}
	}
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("master.key 长度错误")
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "prismflow.db"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS kv (key TEXT PRIMARY KEY, value BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS samples (time INTEGER NOT NULL, protocol TEXT NOT NULL, upload INTEGER NOT NULL, download INTEGER NOT NULL, PRIMARY KEY(time,protocol));
 CREATE TABLE IF NOT EXISTS events (id INTEGER PRIMARY KEY, time TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, message TEXT NOT NULL);
 `)
	if err != nil {
		db.Close()
		return nil, err
	}
	_ = os.Chmod(filepath.Join(dir, "prismflow.db"), 0600)
	return &Store{db, key}, nil
}
func (s *Store) Seal(plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	n := make([]byte, g.NonceSize())
	if _, err = rand.Read(n); err != nil {
		return nil, err
	}
	return g.Seal(n, n, plain, nil), nil
}
func (s *Store) Unseal(b []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(b) < g.NonceSize() {
		return nil, errors.New("加密数据损坏")
	}
	return g.Open(nil, b[:g.NonceSize()], b[g.NonceSize():], nil)
}
func (s *Store) Set(key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b, err = s.Seal(b)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec("INSERT INTO kv(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, b)
	return err
}
func (s *Store) SetMany(values map[string]any) error {
	encoded := map[string][]byte{}
	for k, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		b, err = s.Seal(b)
		if err != nil {
			return err
		}
		encoded[k] = b
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, b := range encoded {
		if _, err = tx.Exec("INSERT INTO kv(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", k, b); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) Get(key string, v any) error {
	var b []byte
	if err := s.DB.QueryRow("SELECT value FROM kv WHERE key=?", key).Scan(&b); err != nil {
		return err
	}
	p, err := s.Unseal(b)
	if err != nil {
		return err
	}
	return json.Unmarshal(p, v)
}
func (s *Store) State() (model.State, error) {
	v := model.Default()
	err := s.Get("state", &v)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return v, err
}
func (s *Store) Event(message string) {
	_, _ = s.DB.Exec("INSERT INTO events(message) VALUES(?)", message)
	_, _ = s.DB.Exec("DELETE FROM events WHERE id NOT IN (SELECT id FROM events ORDER BY id DESC LIMIT 300)")
}
func (s *Store) Events() ([]map[string]string, error) {
	r, err := s.DB.Query("SELECT time,message FROM events ORDER BY id DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer r.Close()
	result := []map[string]string{}
	for r.Next() {
		var t, m string
		if err = r.Scan(&t, &m); err != nil {
			return nil, err
		}
		result = append(result, map[string]string{"time": t, "message": m})
	}
	return result, r.Err()
}
