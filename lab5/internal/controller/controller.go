package controller

import (
	"errors"
	"fmt"
	"lab5/internal/connect"
	"lab5/internal/data"
	"lab5/internal/dns"
	"lab5/internal/handlerRead"
	"lab5/internal/handlerWrite"
	"lab5/internal/utils"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

func Controller() {
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) != 2 {
		fmt.Println("Usage: go run ./main.go <port>")
		return
	}
	port, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("invalid port: %v\n", err)
		return
	}

	lnFD, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	if err != nil {
		fmt.Printf("socket faile: %v\n", err)
		return
	}
	defer func(fd int) {
		err := unix.Close(fd)
		if err != nil {
			log.Printf("close(%d) faile: %v", fd, err)
		}
	}(lnFD)

	_ = unix.SetsockoptInt(lnFD, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
	if err := unix.SetNonblock(lnFD, true); err != nil {
		fmt.Printf("setnonblock faile: %v\n", err)
		return
	}
	sa := &unix.SockaddrInet4{Port: port}
	copy(sa.Addr[:], []byte{0, 0, 0, 0})
	if err := unix.Bind(lnFD, sa); err != nil {
		fmt.Printf("bind faile: %v\n", err)
		return
	}
	if err := unix.Listen(lnFD, 128); err != nil {
		fmt.Printf("listen faile: %v\n", err)
		return
	}
	fmt.Printf("listening on :%d\n", port)

	data.Epfd, err = unix.EpollCreate1(0)
	if err != nil {
		fmt.Printf("epoll_create1 faile: %v\n", err)
		return
	}
	defer func(fd int) {
		err := unix.Close(fd)
		if err != nil {
			log.Printf("close(%d) faile: %v", fd, err)
		}
	}(data.Epfd)

	if err := utils.EpollAdd(lnFD, unix.EPOLLIN); err != nil {
		fmt.Printf("epoll add listen faile: %v\n", err)
		return
	}

	dns.FD, err = unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		fmt.Printf("dns socket faile: %v\n", err)
		return
	}
	defer func(fd int) {
		if fd > 0 {
			err := unix.Close(fd)
			if err != nil {
				log.Printf("close(%d) faile: %v", fd, err)
			}
		}
	}(dns.FD)
	if err := unix.SetNonblock(dns.FD, true); err != nil {
		fmt.Printf("dns setnonblock faile: %v\n", err)
		return
	}
	if err := utils.EpollAdd(dns.FD, unix.EPOLLIN); err != nil {
		fmt.Printf("epoll add dns faile: %v\n", err)
		return
	}

	events := make([]unix.EpollEvent, 128)
	for {
		n, err := unix.EpollWait(data.Epfd, events, -1)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			fmt.Printf("epoll_wait: %v\n", err)
			return
		}
		for i := 0; i < n; i++ {
			ev := events[i]
			fd := int(ev.Fd)
			if fd == lnFD {
				if ev.Events&unix.EPOLLIN != 0 {
					connect.AcceptLoop(lnFD)
				}
				continue
			}

			if fd == dns.FD {
				if ev.Events&unix.EPOLLIN != 0 {
					dns.HandleDNSRead()
				}
				continue
			}

			info := data.FdsInfo[fd]
			if info == nil {
				delete(data.FdsInfo, fd)
				continue
			}
			if ev.Events&(unix.EPOLLHUP|unix.EPOLLERR) != 0 {
				utils.CloseConn(info.Conn)
				continue
			}

			if info.IsClient {
				if ev.Events&unix.EPOLLIN != 0 {
					handlerRead.Client(info.Conn)
				}
				if ev.Events&unix.EPOLLOUT != 0 {
					handlerWrite.Client(info.Conn)
				}
			} else {
				if ev.Events&unix.EPOLLIN != 0 {
					handlerRead.Upstream(info.Conn)
				}
				if ev.Events&unix.EPOLLOUT != 0 {
					handlerWrite.Upstream(info.Conn)
				}
			}
		}
	}
}
