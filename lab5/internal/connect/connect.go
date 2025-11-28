package connect

import (
	"errors"
	"fmt"
	"lab5/internal/data"
	"lab5/internal/handlerWrite"
	"lab5/internal/utils"
	"log"
	"net"

	"golang.org/x/sys/unix"
)

func AcceptLoop(ln int) {
	for {
		nfd, _, err := unix.Accept4(ln, unix.SOCK_NONBLOCK)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				return
			}
			fmt.Printf("accept error: %v\n", err)
			return
		}
		conn := &data.Conn{ClientFD: nfd, UpstreamFD: -1, State: data.StateGreeting}
		data.Conns[nfd] = conn
		data.FdsInfo[nfd] = &data.FDInfo{Conn: conn, IsClient: true}
		if err = utils.EpollAdd(nfd, unix.EPOLLIN); err != nil {
			fmt.Printf("epoll add client: %v\n", err)
			err = unix.Close(nfd)
			if err != nil {
				log.Printf("close(%d) faile: %v", nfd, err)
			}
			delete(data.Conns, nfd)
			delete(data.FdsInfo, nfd)
			continue
		}
	}
}

func StartUpstreamConnect(conn *data.Conn, addr string, port int, isIPv6 bool) bool {
	var upfd int
	var err error

	if isIPv6 {
		upfd, err = unix.Socket(unix.AF_INET6, unix.SOCK_STREAM, 0)
	} else {
		upfd, err = unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	}

	if err != nil {
		utils.SendSocksReply(conn.ClientFD, data.RepGeneralFailure, data.AtypIPv4, nil, 0)
		return false
	}

	conn.UpstreamFD = upfd
	data.FdsInfo[upfd] = &data.FDInfo{Conn: conn, IsClient: false}

	if err = unix.SetNonblock(upfd, true); err != nil {
		err = unix.Close(upfd)
		if err != nil {
			log.Printf("close(%d) faile: %v", upfd, err)
		}
		delete(data.FdsInfo, upfd)
		conn.UpstreamFD = -1
		utils.SendSocksReply(conn.ClientFD, data.RepGeneralFailure, data.AtypIPv4, nil, 0)
		return false
	}

	if err = utils.EpollAdd(upfd, unix.EPOLLOUT); err != nil {
		err = unix.Close(upfd)
		if err != nil {
			log.Printf("close(%d) faile: %v", upfd, err)
		}
		delete(data.FdsInfo, upfd)
		conn.UpstreamFD = -1
		utils.SendSocksReply(conn.ClientFD, data.RepGeneralFailure, data.AtypIPv4, nil, 0)
		return false
	}

	if isIPv6 {
		ip6 := net.ParseIP(addr).To16()
		if ip6 == nil {
			utils.EpollDel(upfd)
			err = unix.Close(upfd)
			if err != nil {
				log.Printf("close(%d) faile: %v", upfd, err)
			}
			delete(data.FdsInfo, upfd)
			conn.UpstreamFD = -1
			utils.SendSocksReply(conn.ClientFD, data.RepGeneralFailure, data.AtypIPv6, nil, 0)
			return false
		}
		var sa unix.SockaddrInet6
		copy(sa.Addr[:], ip6)
		sa.Port = port
		if err = unix.Connect(upfd, &sa); err != nil {
			if errors.Is(err, unix.EINPROGRESS) || errors.Is(err, unix.EALREADY) {
				return true
			}
			utils.EpollDel(upfd)
			err = unix.Close(upfd)
			if err != nil {
				log.Printf("close(%d) faile: %v", upfd, err)
			}
			delete(data.FdsInfo, upfd)
			conn.UpstreamFD = -1
			utils.SendSocksReply(conn.ClientFD, data.RepGeneralFailure, data.AtypIPv6, nil, 0)
			return false
		}
		handlerWrite.Upstream(conn)
		return true
	} else {
		var sa unix.SockaddrInet4
		copy(sa.Addr[:], utils.ParseIPv4(addr)[:])
		sa.Port = port
		if err = unix.Connect(upfd, &sa); err != nil {
			if errors.Is(err, unix.EINPROGRESS) || errors.Is(err, unix.EALREADY) {
				return true
			}
			utils.EpollDel(upfd)
			err = unix.Close(upfd)
			if err != nil {
				log.Printf("close(%d) faile: %v", upfd, err)
			}
			delete(data.FdsInfo, upfd)
			conn.UpstreamFD = -1
			utils.SendSocksReply(conn.ClientFD, data.RepGeneralFailure, data.AtypIPv4, nil, 0)
			return false
		}
		handlerWrite.Upstream(conn)
		return true
	}
}
