package client

import (
	"errors"
	"lab5/internal/data"
	"lab5/internal/utils"

	"golang.org/x/sys/unix"
)

func FlushClientWrites(conn *data.Conn) {
	for conn.UpstreamToClientBuffer.Len() > 0 {
		bytes := conn.UpstreamToClientBuffer.Bytes()
		if len(bytes) == 0 {
			break
		}
		n, err := unix.Write(conn.ClientFD, bytes)
		if n > 0 {
			conn.UpstreamToClientBuffer.Next(n)
		}
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				_ = utils.EpollMod(conn.ClientFD, unix.EPOLLIN|unix.EPOLLOUT)
				return
			}
			utils.CloseConn(conn)
			return
		}
	}
	_ = utils.EpollMod(conn.ClientFD, unix.EPOLLIN)
	if conn.UpstreamClosed && conn.UpstreamToClientBuffer.Len() == 0 {
		_ = unix.Shutdown(conn.ClientFD, unix.SHUT_WR)
	}
}
