package core

import (
	"context"
	"net"
	"syscall"
)

// Windows 的 SO_REUSEADDR 可绕过普通占用；验收使用独占套接字模拟真实冲突。
func occupiedListener() (net.Listener, error) {
	lc := net.ListenConfig{Control: func(_, _ string, raw syscall.RawConn) error {
		var setErr error
		err := raw.Control(func(fd uintptr) { setErr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, -5, 1) })
		if err != nil {
			return err
		}
		return setErr
	}}
	return lc.Listen(context.Background(), "tcp", "0.0.0.0:0")
}
