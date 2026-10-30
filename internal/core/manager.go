package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type Manager struct {
	mu                           sync.Mutex
	Binary, Dir, Address, Secret string
	cmd                          *exec.Cmd
	done                         chan struct{}
	client                       *http.Client
}

func NewManager(binary, dir, address, secret string) *Manager {
	return &Manager{Binary: binary, Dir: dir, Address: address, Secret: secret, client: &http.Client{Timeout: 3 * time.Second}}
}
func (m *Manager) Running() bool { m.mu.Lock(); defer m.mu.Unlock(); return m.running() }
func (m *Manager) running() bool {
	if m.cmd == nil {
		return false
	}
	select {
	case <-m.done:
		return false
	default:
		return true
	}
}
func (m *Manager) Available() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := exec.LookPath(m.Binary)
	return err == nil
}
func (m *Manager) BinaryPath() string    { m.mu.Lock(); defer m.mu.Unlock(); return m.Binary }
func (m *Manager) SetBinary(path string) { m.mu.Lock(); defer m.mu.Unlock(); m.Binary = path }
func (m *Manager) request(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+m.Address+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.Secret)
	res, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("内核接口返回 %d", res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(v)
}
func (m *Manager) Request(ctx context.Context, path string, v any) error {
	return m.request(ctx, path, v)
}
func (m *Manager) Validate(ctx context.Context, b []byte) error {
	if _, lookupErr := exec.LookPath(m.Binary); lookupErr != nil {
		return errors.New("未找到 Mihomo；请先安装锁定版本的内核")
	}
	if err := os.MkdirAll(m.Dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(m.Dir, "candidate-*.yaml")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.Binary, "-t", "-d", m.Dir, "-f", path)
	if err = cmd.Run(); err != nil {
		return errors.New("Mihomo 配置校验失败；请检查协议组合、证书及内核版本")
	}
	return nil
}
func (m *Manager) stop() {
	if m.running() {
		_ = m.cmd.Process.Kill()
		<-m.done
	}
	m.cmd = nil
}
func (m *Manager) Stop() { m.mu.Lock(); defer m.mu.Unlock(); m.stop() }
func (m *Manager) start(ctx context.Context, path string) error {
	startup, err := newStartupStatus(path)
	if err != nil {
		return err
	}
	defer startup.close()
	cmd := exec.Command(m.Binary, "-d", m.Dir, "-f", path)
	cmd.Stdout = startup
	cmd.Stderr = startup
	if err := cmd.Start(); err != nil {
		return err
	}
	m.cmd = cmd
	m.done = make(chan struct{})
	done := m.done
	go func() { _ = cmd.Wait(); close(done) }()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			m.stop()
			return ctx.Err()
		case <-deadline.C:
			m.stop()
			return errors.New("内核未在 10 秒内就绪")
		case <-done:
			return errors.New("内核启动退出；请检查端口占用和配置")
		case <-tick.C:
			ready, failed := startup.status()
			if failed {
				m.stop()
				return errors.New("协议入口监听失败，请检查端口占用和权限")
			}
			if !ready {
				continue
			}
			var v struct {
				Version string `json:"version"`
			}
			if m.request(ctx, "/version", &v) == nil {
				return nil
			}
		}
	}
}

// Publish 在进程验证成功之后提交应用状态，提交失败则恢复旧进程配置。
func (m *Manager) Publish(ctx context.Context, b []byte, commit func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.Validate(ctx, b); err != nil {
		return err
	}
	path := filepath.Join(m.Dir, "active.yaml")
	old, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	wasRunning := m.running()
	if len(old) > 0 {
		if err := os.WriteFile(filepath.Join(m.Dir, "previous.yaml"), old, 0600); err != nil {
			return err
		}
	}
	m.stop()
	err := os.WriteFile(path, b, 0600)
	if err == nil {
		err = m.start(ctx, path)
	}
	if err == nil {
		err = commit()
	}
	if err == nil {
		return nil
	}
	m.stop()
	if len(old) > 0 {
		if restoreErr := os.WriteFile(path, old, 0600); restoreErr != nil {
			return fmt.Errorf("发布失败且恢复文件失败：%w", restoreErr)
		}
		if wasRunning {
			restoreCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()
			if restoreErr := m.start(restoreCtx, path); restoreErr != nil {
				return fmt.Errorf("发布失败且旧内核恢复失败：%w", restoreErr)
			}
		}
	} else {
		_ = os.Remove(path)
	}
	return fmt.Errorf("发布失败，已恢复原配置：%w", err)
}

// 检测结果只对同一个服务进程有效。
func (m *Manager) InstanceID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running() {
		return 0
	}
	return m.cmd.Process.Pid
}
