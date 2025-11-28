package handlerRead

import (
	"errors"
	"lab5/internal/client"
	"lab5/internal/data"
	"lab5/internal/handshake"
	"lab5/internal/upStream"
	"lab5/internal/utils"

	"golang.org/x/sys/unix"
)

func Client(conn *data.Conn) {
	fd := conn.ClientFD
	buf := make([]byte, 32*1024)
	for {
		n, err := unix.Read(fd, buf)
		if n > 0 {
			if conn.State == data.StateGreeting || conn.State == data.StateRequest {
				conn.HandshakeBuffer.Write(buf[:n])
				handshake.TryProcessHandshake(conn)
			} else {
				conn.ClientToUpstreamBuffer.Write(buf[:n])
				upStream.FlushUpstreamWrites(conn)
			}
		}
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				return
			}
			utils.CloseConn(conn)
			return
		}
		if n == 0 {
			conn.ClientClosed = true
			if conn.UpstreamFD >= 0 && conn.ClientToUpstreamBuffer.Len() == 0 {
				_ = unix.Shutdown(conn.UpstreamFD, unix.SHUT_WR)
			}
			return
		}
	}
}

func Upstream(conn *data.Conn) {
	fd := conn.UpstreamFD
	if fd < 0 {
		return
	}
	buf := make([]byte, 32*1024)
	for {
		n, err := unix.Read(fd, buf)
		if n > 0 {
			conn.UpstreamToClientBuffer.Write(buf[:n])
			client.FlushClientWrites(conn)
		}
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				return
			}
			utils.CloseConn(conn)
			return
		}
		if n == 0 {
			conn.UpstreamClosed = true
			if conn.UpstreamToClientBuffer.Len() == 0 {
				_ = unix.Shutdown(conn.ClientFD, unix.SHUT_WR)
			}
			return
		}
	}
}
