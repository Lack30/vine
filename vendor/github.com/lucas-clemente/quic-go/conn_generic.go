// +build !darwin,!linux

package quic

import "net"

func newConn(c net.PacketConn) (connection, error) {
	return &basicConn{PacketConn: c}, nil
}

func inspectReadBuffer(net.PacketConn) (int, error) {
	return 0, nil
}
