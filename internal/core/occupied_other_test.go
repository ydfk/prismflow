//go:build !windows

package core

import "net"

func occupiedListener() (net.Listener, error) { return net.Listen("tcp", "0.0.0.0:0") }
