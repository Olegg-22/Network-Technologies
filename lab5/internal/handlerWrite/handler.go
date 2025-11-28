package handlerWrite

import (
	"lab5/internal/client"
	"lab5/internal/data"
	"lab5/internal/upStream"
	"lab5/internal/utils"

	"golang.org/x/sys/unix"
)

func Client(conn *data.Conn) {
	client.FlushClientWrites(conn)
}

func Upstream(conn *data.Conn) {
	upfd := conn.UpstreamFD
	if upfd < 0 {
		return
	}
	if conn.State == data.StateConnecting {
		soErr, err := unix.GetsockoptInt(upfd, unix.SOL_SOCKET, unix.SO_ERROR)
		if err != nil || soErr != 0 {
			utils.SendSocksReply(conn.ClientFD, data.RepGeneralFailure, data.AtypIPv4, nil, 0)
			utils.CloseConn(conn)
			return
		}
		sa, err := unix.Getsockname(conn.UpstreamFD)
		if err != nil {
			utils.SendSocksReply(conn.ClientFD, data.RepGeneralFailure, data.AtypIPv4, nil, 0)
			utils.CloseConn(conn)
			return
		}
		if _, isIPv6 := sa.(*unix.SockaddrInet6); isIPv6 {
			if !utils.SendSocksReply(conn.ClientFD, data.RepSuccess, data.AtypIPv6, make([]byte, 16), 0) {
				utils.CloseConn(conn)
				return
			}
		} else {
			if !utils.SendSocksReply(conn.ClientFD, data.RepSuccess, data.AtypIPv4, []byte{0, 0, 0, 0}, 0) {
				utils.CloseConn(conn)
				return
			}
		}
		_ = utils.EpollMod(upfd, unix.EPOLLIN)
		_ = utils.EpollMod(conn.ClientFD, unix.EPOLLIN)
		conn.State = data.StateRelaying
		upStream.FlushUpstreamWrites(conn)
		return
	}
	upStream.FlushUpstreamWrites(conn)
}
