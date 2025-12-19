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
	var upstreamFd int
	var err error

	if isIPv6 {
		upstreamFd, err = unix.Socket(unix.AF_INET6, unix.SOCK_STREAM, 0)
	} else {
		upstreamFd, err = unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	}

	if err != nil {
		var atyp byte
		if isIPv6 {
			atyp = data.AtypIPv6
		} else {
			atyp = data.AtypIPv4
		}
		utils.SendSocksReply(conn, data.RepGeneralFailure, atyp, nil, 0)
		return false
	}

	conn.UpstreamFD = upstreamFd
	data.FdsInfo[upstreamFd] = &data.FDInfo{Conn: conn, IsClient: false}

	if err = unix.SetNonblock(upstreamFd, true); err != nil {
		err = unix.Close(upstreamFd)
		if err != nil {
			log.Printf("close(%d) faile: %v", upstreamFd, err)
		}
		delete(data.FdsInfo, upstreamFd)
		conn.UpstreamFD = -1
		utils.SendSocksReply(conn, data.RepGeneralFailure, data.AtypIPv4, nil, 0)
		return false
	}

	if err = utils.EpollAdd(upstreamFd, unix.EPOLLOUT); err != nil {
		err = unix.Close(upstreamFd)
		if err != nil {
			log.Printf("close(%d) faile: %v", upstreamFd, err)
		}
		delete(data.FdsInfo, upstreamFd)
		conn.UpstreamFD = -1
		utils.SendSocksReply(conn, data.RepGeneralFailure, data.AtypIPv4, nil, 0)
		return false
	}

	var ipAddr []byte
	if isIPv6 {
		ipAddr = net.ParseIP(addr).To16()
	} else {
		ipAddr = net.ParseIP(addr).To4()
	}

	if ipAddr == nil {
		utils.EpollDel(upstreamFd)
		err = unix.Close(upstreamFd)
		if err != nil {
			log.Printf("close(%d) faile: %v", upstreamFd, err)
		}
		delete(data.FdsInfo, upstreamFd)
		conn.UpstreamFD = -1
		var atyp byte
		if isIPv6 {
			atyp = data.AtypIPv6
		} else {
			atyp = data.AtypIPv4
		}
		utils.SendSocksReply(conn, data.RepGeneralFailure, atyp, nil, 0)
		return false
	}

	var sa unix.Sockaddr
	if isIPv6 {
		sa6 := &unix.SockaddrInet6{
			Port: port,
		}
		copy(sa6.Addr[:], ipAddr)
		sa = sa6
	} else {
		sa4 := &unix.SockaddrInet4{
			Port: port,
		}
		copy(sa4.Addr[:], ipAddr)
		sa = sa4
	}

	if err = unix.Connect(upstreamFd, sa); err != nil {
		if errors.Is(err, unix.EINPROGRESS) || errors.Is(err, unix.EALREADY) {
			return true
		}
		utils.EpollDel(upstreamFd)
		err = unix.Close(upstreamFd)
		if err != nil {
			log.Printf("close(%d) faile: %v", upstreamFd, err)
		}
		delete(data.FdsInfo, upstreamFd)
		conn.UpstreamFD = -1
		var atyp byte
		if isIPv6 {
			atyp = data.AtypIPv6
		} else {
			atyp = data.AtypIPv4
		}
		utils.SendSocksReply(conn, data.RepGeneralFailure, atyp, nil, 0)
		return false
	}
	handlerWrite.Upstream(conn)
	return true
}
