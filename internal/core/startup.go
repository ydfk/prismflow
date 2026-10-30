package core

import (
	"bytes"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// 控制器可能早于监听器就绪。仅解析锁定内核的启动标记，不保存或输出原始日志。
type startupStatus struct {
	mu      sync.Mutex
	waiting map[string]bool
	partial []byte
	failed  bool
	closed  bool
}

func newStartupStatus(path string) (*startupStatus, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config struct {
		MixedPort int `yaml:"mixed-port"`
		Listeners []struct {
			Name   string `yaml:"name"`
			Type   string `yaml:"type"`
			Target string `yaml:"target"`
		} `yaml:"listeners"`
	}
	if err = yaml.Unmarshal(b, &config); err != nil {
		return nil, err
	}
	s := &startupStatus{waiting: map[string]bool{}}
	for _, l := range config.Listeners {
		marker := "[" + l.Name + "] proxy listening at:"
		if l.Type == "tunnel" {
			marker = "[" + l.Name + "](" + l.Target + ")proxy listening at:"
		}
		s.waiting[marker] = true
	}
	if config.MixedPort > 0 {
		s.waiting["Mixed(http+socks) proxy listening at:"] = true
	}
	return s, nil
}

func (s *startupStatus) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return len(p), nil
	}
	for _, b := range p {
		if b != '\n' {
			if len(s.partial) < 8192 {
				s.partial = append(s.partial, b)
			}
			continue
		}
		line := string(s.partial)
		if strings.Contains(line, "listen err:") || strings.Contains(line, "Start Mixed(http+socks) server error:") {
			s.failed = true
		}
		for name := range s.waiting {
			if bytes.Contains(s.partial, []byte(name)) {
				delete(s.waiting, name)
			}
		}
		s.partial = s.partial[:0]
	}
	return len(p), nil
}

func (s *startupStatus) status() (ready, failed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.waiting) == 0, s.failed
}
func (s *startupStatus) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.partial = nil
	s.waiting = nil
}
