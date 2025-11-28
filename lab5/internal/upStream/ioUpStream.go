package upStream

import (
	"errors"
	"lab5/internal/data"
	"lab5/internal/utils"

	"golang.org/x/sys/unix"
)

func FlushUpstreamWrites(conn *data.Conn) {
	if conn.UpstreamFD < 0 {
		return
	}
	for conn.ClientToUpstreamBuffer.Len() > 0 {
		bytes := conn.ClientToUpstreamBuffer.Bytes()
		if len(bytes) == 0 {
			break
		}
		n, err := unix.Write(conn.UpstreamFD, bytes)
		if n > 0 {
			conn.ClientToUpstreamBuffer.Next(n)
		}
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				_ = utils.EpollMod(conn.UpstreamFD, unix.EPOLLIN|unix.EPOLLOUT)
				return
			}
			utils.CloseConn(conn)
			return
		}
	}
	_ = utils.EpollMod(conn.UpstreamFD, unix.EPOLLIN)
	if conn.ClientClosed && conn.ClientToUpstreamBuffer.Len() == 0 {
		_ = unix.Shutdown(conn.UpstreamFD, unix.SHUT_WR)
	}
}
