package utils

import (
	"encoding/binary"
	"errors"
	"lab5/internal/data"
	"log"

	"golang.org/x/sys/unix"
)

func EpollAdd(fd int, events uint32) error {
	ev := &unix.EpollEvent{Events: events, Fd: int32(fd)}
	return unix.EpollCtl(data.Epfd, unix.EPOLL_CTL_ADD, fd, ev)
}
func EpollMod(fd int, events uint32) error {
	ev := &unix.EpollEvent{Events: events, Fd: int32(fd)}
	return unix.EpollCtl(data.Epfd, unix.EPOLL_CTL_MOD, fd, ev)
}
func EpollDel(fd int) { _ = unix.EpollCtl(data.Epfd, unix.EPOLL_CTL_DEL, fd, nil) }

func CloseConn(conn *data.Conn) {
	if conn == nil {
		return
	}
	if conn.ClientFD >= 0 {
		EpollDel(conn.ClientFD)
		err := unix.Close(conn.ClientFD)
		if err != nil {
			log.Printf("close(%d) faile: %v", conn.ClientFD, err)
		}
		delete(data.FdsInfo, conn.ClientFD)
		delete(data.Conns, conn.ClientFD)
		conn.ClientFD = -1
	}
	if conn.UpstreamFD >= 0 {
		EpollDel(conn.UpstreamFD)
		err := unix.Close(conn.UpstreamFD)
		if err != nil {
			log.Printf("close(%d) faile: %v", conn.UpstreamFD, err)
		}
		delete(data.FdsInfo, conn.UpstreamFD)
		conn.UpstreamFD = -1
	}
}

func SendSocksReply(clientFD int, rep byte, atyp byte, bndAddr []byte, bndPort int) bool {
	if bndAddr == nil {
		bndAddr = []byte{0, 0, 0, 0}
	}
	resp := []byte{data.SocksVer, rep, 0x00, atyp}
	resp = append(resp, bndAddr...)
	portb := make([]byte, 2)
	binary.BigEndian.PutUint16(portb, uint16(bndPort))
	resp = append(resp, portb...)
	return WriteAll(clientFD, resp)
}

func WriteAll(fd int, data []byte) bool {
	off := 0
	for off < len(data) {
		n, err := unix.Write(fd, data[off:])
		if n > 0 {
			off += n
		}
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				return false
			}
			return false
		}
	}
	return true
}
