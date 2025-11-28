package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

const (
	socksVer = 0x05

	socksMethodNoAuth = 0x00

	socksCmdConnect = 0x01

	atypIPv4   = 0x01
	atypDomain = 0x03
	atypIPv6   = 0x04

	repSuccess              = 0x00
	repGeneralFailure       = 0x01
	repCommandNotSupported  = 0x07
	repAddrTypeNotSupported = 0x08
)

const (
	StateGreeting   = 0
	StateRequest    = 1
	StateConnecting = 2
	StateRelaying   = 3
	StateResolving  = 4
)

type Conn struct {
	clientFD   int
	upstreamFD int

	handshake bytes.Buffer

	c2u bytes.Buffer
	u2c bytes.Buffer

	state          int
	clientClosed   bool
	upstreamClosed bool
}

type FDInfo struct {
	conn     *Conn
	isClient bool
}

type pendingResolve struct {
	conn   *Conn
	domain string
	port   int
}

var (
	epfd            int
	fd2info             = make(map[int]*FDInfo)
	conns               = make(map[int]*Conn)
	dnsFD           int = -1
	pendingResolves     = make(map[uint16]*pendingResolve)
	dnsResolverAddr     = &unix.SockaddrInet4{Port: 53, Addr: [4]byte{8, 8, 8, 8}}
)

func epollAdd(fd int, events uint32) error {
	ev := &unix.EpollEvent{Events: events, Fd: int32(fd)}
	return unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, fd, ev)
}
func epollMod(fd int, events uint32) error {
	ev := &unix.EpollEvent{Events: events, Fd: int32(fd)}
	return unix.EpollCtl(epfd, unix.EPOLL_CTL_MOD, fd, ev)
}
func epollDel(fd int) { _ = unix.EpollCtl(epfd, unix.EPOLL_CTL_DEL, fd, nil) }

func main() {
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

	epfd, err = unix.EpollCreate1(0)
	if err != nil {
		fmt.Printf("epoll_create1 faile: %v\n", err)
		return
	}
	defer func(fd int) {
		err := unix.Close(fd)
		if err != nil {
			log.Printf("close(%d) faile: %v", fd, err)
		}
	}(epfd)

	if err := epollAdd(lnFD, unix.EPOLLIN); err != nil {
		fmt.Printf("epoll add listen faile: %v\n", err)
		return
	}

	dnsFD, err = unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
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
	}(dnsFD)
	if err := unix.SetNonblock(dnsFD, true); err != nil {
		fmt.Printf("dns setnonblock faile: %v\n", err)
		return
	}
	if err := epollAdd(dnsFD, unix.EPOLLIN); err != nil {
		fmt.Printf("epoll add dns faile: %v\n", err)
		return
	}

	events := make([]unix.EpollEvent, 128)
	for {
		n, err := unix.EpollWait(epfd, events, -1)
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
					acceptLoop(lnFD)
				}
				continue
			}

			if fd == dnsFD {
				if ev.Events&unix.EPOLLIN != 0 {
					handleDNSRead()
				}
				continue
			}

			info := fd2info[fd]
			if info == nil {
				delete(fd2info, fd)
				continue
			}
			if ev.Events&(unix.EPOLLHUP|unix.EPOLLERR) != 0 {
				closeConn(info.conn)
				continue
			}

			if info.isClient {
				if ev.Events&unix.EPOLLIN != 0 {
					handleClientRead(info.conn)
				}
				if ev.Events&unix.EPOLLOUT != 0 {
					handleClientWrite(info.conn)
				}
			} else {
				if ev.Events&unix.EPOLLIN != 0 {
					handleUpstreamRead(info.conn)
				}
				if ev.Events&unix.EPOLLOUT != 0 {
					handleUpstreamWrite(info.conn)
				}
			}
		}
	}
}

func acceptLoop(ln int) {
	for {
		nfd, _, err := unix.Accept4(ln, unix.SOCK_NONBLOCK)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				return
			}
			fmt.Printf("accept error: %v\n", err)
			return
		}
		c := &Conn{clientFD: nfd, upstreamFD: -1, state: StateGreeting}
		conns[nfd] = c
		fd2info[nfd] = &FDInfo{conn: c, isClient: true}
		if err = epollAdd(nfd, unix.EPOLLIN); err != nil {
			fmt.Printf("epoll add client: %v\n", err)
			err = unix.Close(nfd)
			if err != nil {
				log.Printf("close(%d) faile: %v", nfd, err)
			}
			delete(conns, nfd)
			delete(fd2info, nfd)
			continue
		}
	}
}

func handleClientRead(c *Conn) {
	fd := c.clientFD
	buf := make([]byte, 32*1024)
	for {
		n, err := unix.Read(fd, buf)
		if n > 0 {
			if c.state == StateGreeting || c.state == StateRequest {
				c.handshake.Write(buf[:n])
				tryProcessHandshake(c)
			} else {
				c.c2u.Write(buf[:n])
				flushUpstreamWrites(c)
			}
		}
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				return
			}
			closeConn(c)
			return
		}
		if n == 0 {
			c.clientClosed = true
			if c.upstreamFD >= 0 && c.c2u.Len() == 0 {
				_ = unix.Shutdown(c.upstreamFD, unix.SHUT_WR)
			}
			return
		}
	}
}

func tryProcessHandshake(c *Conn) {
	for {
		switch c.state {
		case StateGreeting:
			if c.handshake.Len() < 2 {
				return
			}
			b := c.handshake.Bytes()
			if b[0] != socksVer {
				closeConn(c)
				return
			}
			nm := int(b[1])
			if c.handshake.Len() < 2+nm {
				return
			}
			methods := b[2 : 2+nm]
			ok := false
			for _, m := range methods {
				if m == socksMethodNoAuth {
					ok = true
					break
				}
			}
			c.handshake.Next(2 + nm)

			if !writeAll(c.clientFD, []byte{socksVer, socksMethodNoAuth}) {
				closeConn(c)
				return
			}
			if !ok {
				closeConn(c)
				return
			}
			c.state = StateRequest
		case StateRequest:
			if c.handshake.Len() < 4 {
				return
			}
			b := c.handshake.Bytes()
			if b[0] != socksVer {
				closeConn(c)
				return
			}
			cmd := b[1]
			atyp := b[3]
			if cmd != socksCmdConnect {
				sendSocksReply(c.clientFD, repCommandNotSupported, atyp, nil, 0)
				closeConn(c)
				return
			}
			if atyp == atypIPv4 {
				if c.handshake.Len() < 10 {
					return
				}
				addr := fmt.Sprintf("%d.%d.%d.%d", b[4], b[5], b[6], b[7])
				port := int(binary.BigEndian.Uint16(b[8:10]))
				c.handshake.Next(10)
				if !startUpstreamConnect(c, addr, port, false) {
					closeConn(c)
					return
				}
				c.state = StateConnecting
				return
			}

			if atyp == atypDomain {
				if c.handshake.Len() < 5 {
					return
				}
				dlen := int(b[4])
				if c.handshake.Len() < 5+dlen+2 {
					return
				}
				domain := string(b[5 : 5+dlen])
				port := int(binary.BigEndian.Uint16(b[5+dlen : 5+dlen+2]))

				c.handshake.Next(5 + dlen + 2)

				pr := &pendingResolve{conn: c, domain: domain, port: port}
				_, err := sendDNSQuery(domain, pr)
				if err != nil {
					sendSocksReply(c.clientFD, repGeneralFailure, atypDomain, nil, 0)
					closeConn(c)
					return
				}
				c.state = StateResolving
				return
			}

			if atyp == atypIPv6 {
				if c.handshake.Len() < 4+16+2 {
					return
				}
				addrBytes := b[4 : 4+16]
				port := int(binary.BigEndian.Uint16(b[4+16 : 4+16+2]))
				c.handshake.Next(4 + 16 + 2)
				domainOrIP := net.IP(addrBytes).String()
				if !startUpstreamConnect(c, domainOrIP, port, true) {
					closeConn(c)
					return
				}
				c.state = StateConnecting
				return
			}

			sendSocksReply(c.clientFD, repAddrTypeNotSupported, atyp, nil, 0)
			closeConn(c)
			return
		default:
			return
		}
	}
}

func startUpstreamConnect(c *Conn, addr string, port int, isIPv6 bool) bool {
	var upfd int
	var err error

	if isIPv6 {
		upfd, err = unix.Socket(unix.AF_INET6, unix.SOCK_STREAM, 0)
	} else {
		upfd, err = unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	}

	if err != nil {
		sendSocksReply(c.clientFD, repGeneralFailure, atypIPv4, nil, 0)
		return false
	}

	c.upstreamFD = upfd
	fd2info[upfd] = &FDInfo{conn: c, isClient: false}

	if err = unix.SetNonblock(upfd, true); err != nil {
		err = unix.Close(upfd)
		if err != nil {
			log.Printf("close(%d) faile: %v", upfd, err)
		}
		delete(fd2info, upfd)
		c.upstreamFD = -1
		sendSocksReply(c.clientFD, repGeneralFailure, atypIPv4, nil, 0)
		return false
	}

	if err = epollAdd(upfd, unix.EPOLLOUT); err != nil {
		err = unix.Close(upfd)
		if err != nil {
			log.Printf("close(%d) faile: %v", upfd, err)
		}
		delete(fd2info, upfd)
		c.upstreamFD = -1
		sendSocksReply(c.clientFD, repGeneralFailure, atypIPv4, nil, 0)
		return false
	}

	if isIPv6 {
		ip6 := net.ParseIP(addr).To16()
		if ip6 == nil {
			epollDel(upfd)
			err = unix.Close(upfd)
			if err != nil {
				log.Printf("close(%d) faile: %v", upfd, err)
			}
			delete(fd2info, upfd)
			c.upstreamFD = -1
			sendSocksReply(c.clientFD, repGeneralFailure, atypIPv6, nil, 0)
			return false
		}
		var sa unix.SockaddrInet6
		copy(sa.Addr[:], ip6)
		sa.Port = port
		if err = unix.Connect(upfd, &sa); err != nil {
			if errors.Is(err, unix.EINPROGRESS) || errors.Is(err, unix.EALREADY) {
				return true
			}
			epollDel(upfd)
			err = unix.Close(upfd)
			if err != nil {
				log.Printf("close(%d) faile: %v", upfd, err)
			}
			delete(fd2info, upfd)
			c.upstreamFD = -1
			sendSocksReply(c.clientFD, repGeneralFailure, atypIPv6, nil, 0)
			return false
		}
		handleUpstreamWrite(c)
		return true
	} else {
		var sa unix.SockaddrInet4
		copy(sa.Addr[:], parseIPv4(addr)[:])
		sa.Port = port
		if err = unix.Connect(upfd, &sa); err != nil {
			if errors.Is(err, unix.EINPROGRESS) || errors.Is(err, unix.EALREADY) {
				return true
			}
			epollDel(upfd)
			err = unix.Close(upfd)
			if err != nil {
				log.Printf("close(%d) faile: %v", upfd, err)
			}
			delete(fd2info, upfd)
			c.upstreamFD = -1
			sendSocksReply(c.clientFD, repGeneralFailure, atypIPv4, nil, 0)
			return false
		}
		handleUpstreamWrite(c)
		return true
	}
}

func handleUpstreamWrite(c *Conn) {
	upfd := c.upstreamFD
	if upfd < 0 {
		return
	}
	if c.state == StateConnecting {
		soErr, err := unix.GetsockoptInt(upfd, unix.SOL_SOCKET, unix.SO_ERROR)
		if err != nil || soErr != 0 {
			sendSocksReply(c.clientFD, repGeneralFailure, atypIPv4, nil, 0)
			closeConn(c)
			return
		}
		sa, err := unix.Getsockname(c.upstreamFD)
		if err != nil {
			sendSocksReply(c.clientFD, repGeneralFailure, atypIPv4, nil, 0)
			closeConn(c)
			return
		}
		if _, isIPv6 := sa.(*unix.SockaddrInet6); isIPv6 {
			if !sendSocksReply(c.clientFD, repSuccess, atypIPv6, make([]byte, 16), 0) {
				closeConn(c)
				return
			}
		} else {
			if !sendSocksReply(c.clientFD, repSuccess, atypIPv4, []byte{0, 0, 0, 0}, 0) {
				closeConn(c)
				return
			}
		}
		_ = epollMod(upfd, unix.EPOLLIN)
		_ = epollMod(c.clientFD, unix.EPOLLIN)
		c.state = StateRelaying
		flushUpstreamWrites(c)
		return
	}
	flushUpstreamWrites(c)
}

func handleUpstreamRead(c *Conn) {
	fd := c.upstreamFD
	if fd < 0 {
		return
	}
	buf := make([]byte, 32*1024)
	for {
		n, err := unix.Read(fd, buf)
		if n > 0 {
			c.u2c.Write(buf[:n])
			flushClientWrites(c)
		}
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				return
			}
			closeConn(c)
			return
		}
		if n == 0 {
			c.upstreamClosed = true
			if c.u2c.Len() == 0 {
				_ = unix.Shutdown(c.clientFD, unix.SHUT_WR)
			}
			return
		}
	}
}

func flushUpstreamWrites(c *Conn) {
	if c.upstreamFD < 0 {
		return
	}
	for c.c2u.Len() > 0 {
		data := c.c2u.Bytes()
		if len(data) == 0 {
			break
		}
		n, err := unix.Write(c.upstreamFD, data)
		if n > 0 {
			c.c2u.Next(n)
		}
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				_ = epollMod(c.upstreamFD, unix.EPOLLIN|unix.EPOLLOUT)
				return
			}
			closeConn(c)
			return
		}
	}
	_ = epollMod(c.upstreamFD, unix.EPOLLIN)
	if c.clientClosed && c.c2u.Len() == 0 {
		_ = unix.Shutdown(c.upstreamFD, unix.SHUT_WR)
	}
}

func handleClientWrite(c *Conn) {
	flushClientWrites(c)
}

func flushClientWrites(c *Conn) {
	for c.u2c.Len() > 0 {
		data := c.u2c.Bytes()
		if len(data) == 0 {
			break
		}
		n, err := unix.Write(c.clientFD, data)
		if n > 0 {
			c.u2c.Next(n)
		}
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				_ = epollMod(c.clientFD, unix.EPOLLIN|unix.EPOLLOUT)
				return
			}
			closeConn(c)
			return
		}
	}
	_ = epollMod(c.clientFD, unix.EPOLLIN)
	if c.upstreamClosed && c.u2c.Len() == 0 {
		_ = unix.Shutdown(c.clientFD, unix.SHUT_WR)
	}
}

func sendSocksReply(clientFD int, rep byte, atyp byte, bndAddr []byte, bndPort int) bool {
	if bndAddr == nil {
		bndAddr = []byte{0, 0, 0, 0}
	}
	resp := []byte{socksVer, rep, 0x00, atyp}
	resp = append(resp, bndAddr...)
	portb := make([]byte, 2)
	binary.BigEndian.PutUint16(portb, uint16(bndPort))
	resp = append(resp, portb...)
	return writeAll(clientFD, resp)
}

func writeAll(fd int, data []byte) bool {
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

func closeConn(c *Conn) {
	if c == nil {
		return
	}
	if c.clientFD >= 0 {
		epollDel(c.clientFD)
		err := unix.Close(c.clientFD)
		if err != nil {
			log.Printf("close(%d) faile: %v", c.clientFD, err)
		}
		delete(fd2info, c.clientFD)
		delete(conns, c.clientFD)
		c.clientFD = -1
	}
	if c.upstreamFD >= 0 {
		epollDel(c.upstreamFD)
		err := unix.Close(c.upstreamFD)
		if err != nil {
			log.Printf("close(%d) faile: %v", c.upstreamFD, err)
		}
		delete(fd2info, c.upstreamFD)
		c.upstreamFD = -1
	}
}

func buildDNSQuery(id uint16, name string) ([]byte, error) {
	dnsQuery := &bytes.Buffer{}
	_ = binary.Write(dnsQuery, binary.BigEndian, id)
	_ = binary.Write(dnsQuery, binary.BigEndian, uint16(0x0100))
	_ = binary.Write(dnsQuery, binary.BigEndian, uint16(1))
	_ = binary.Write(dnsQuery, binary.BigEndian, uint16(0))
	_ = binary.Write(dnsQuery, binary.BigEndian, uint16(0))
	_ = binary.Write(dnsQuery, binary.BigEndian, uint16(0))

	for _, label := range bytes.Split([]byte(name), []byte{'.'}) {
		if len(label) == 0 || len(label) > 63 {
			return nil, fmt.Errorf("invalid dns name")
		}
		dnsQuery.WriteByte(byte(len(label)))
		dnsQuery.Write(label)
	}
	dnsQuery.WriteByte(0)
	_ = binary.Write(dnsQuery, binary.BigEndian, uint16(1))
	_ = binary.Write(dnsQuery, binary.BigEndian, uint16(1))
	return dnsQuery.Bytes(), nil
}

func parseDNSResponse(dnsResponse []byte) (uint16, string, error) {
	if len(dnsResponse) < 12 {
		return 0, "", fmt.Errorf("short dns response")
	}
	id := binary.BigEndian.Uint16(dnsResponse[0:2])
	flags := binary.BigEndian.Uint16(dnsResponse[2:4])
	if (flags>>15)&1 == 0 {
		return id, "", fmt.Errorf("not a response")
	}
	rcode := flags & 0xF
	if rcode != 0 {
		return id, "", fmt.Errorf("rcode=%d", rcode)
	}
	qdcount := int(binary.BigEndian.Uint16(dnsResponse[4:6]))
	ancount := int(binary.BigEndian.Uint16(dnsResponse[6:8]))
	off := 12
	for i := 0; i < qdcount; i++ {
		for {
			if off >= len(dnsResponse) {
				return id, "", fmt.Errorf("uncorrect response")
			}
			l := int(dnsResponse[off])
			off++
			if l == 0 {
				break
			}
			off += l
		}
		off += 4
	}
	for i := 0; i < ancount; i++ {
		if off+10 > len(dnsResponse) {
			return id, "", fmt.Errorf("short answer")
		}
		if dnsResponse[off]&0xC0 == 0xC0 {
			off += 2
		} else {
			for {
				if off >= len(dnsResponse) {
					return id, "", fmt.Errorf("uncorrect answer name")
				}
				l := int(dnsResponse[off])
				off++
				if l == 0 {
					break
				}
				off += l
			}
		}
		typ := binary.BigEndian.Uint16(dnsResponse[off : off+2])
		off += 2
		class := binary.BigEndian.Uint16(dnsResponse[off : off+2])
		off += 2 + 4
		rdlen := int(binary.BigEndian.Uint16(dnsResponse[off : off+2]))
		off += 2
		if off+rdlen > len(dnsResponse) {
			return id, "", fmt.Errorf("rdata out of bounds")
		}
		rdata := dnsResponse[off : off+rdlen]
		off += rdlen
		if typ == 1 && class == 1 && rdlen == 4 {
			ip := net.IPv4(rdata[0], rdata[1], rdata[2], rdata[3]).String()
			return id, ip, nil
		}
	}
	return id, "", fmt.Errorf("no A record found")
}

func sendDNSQuery(domain string, p *pendingResolve) (uint16, error) {
	var id uint16
	for tries := 0; tries < 10; tries++ {
		id = uint16(rand.Intn(0xffff))
		if _, exists := pendingResolves[id]; !exists {
			break
		}
		if tries == 9 {
			return 0, fmt.Errorf("faile to find free dns id")
		}
	}

	dnsQuery, err := buildDNSQuery(id, domain)
	if err != nil {
		return 0, err
	}

	if err := unix.Sendto(dnsFD, dnsQuery, 0, dnsResolverAddr); err != nil {
		return 0, err
	}

	pendingResolves[id] = p
	return id, nil
}

func handleDNSRead() {
	buf := make([]byte, 4*1024)
	for {
		n, _, err := unix.Recvfrom(dnsFD, buf, 0)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				return
			}
			fmt.Printf("dns recvfrom: %v\n", err)
			return
		}
		if n == 0 {
			return
		}

		id, ipStr, err := parseDNSResponse(buf[:n])
		if err != nil {
			fmt.Printf("dns parse err: %v\n", err)
			continue
		}

		pendingRequest := pendingResolves[id]
		if pendingRequest == nil {
			continue
		}
		delete(pendingResolves, id)

		ip := net.ParseIP(ipStr)
		if ip == nil {
			sendSocksReply(pendingRequest.conn.clientFD, repGeneralFailure, atypDomain, nil, 0)
			closeConn(pendingRequest.conn)
			continue
		}
		isIPv6 := false
		if ip.To4() == nil {
			isIPv6 = true
		}

		if !startUpstreamConnect(pendingRequest.conn, ipStr, pendingRequest.port, isIPv6) {
			closeConn(pendingRequest.conn)
			continue
		}
		pendingRequest.conn.state = StateConnecting
	}
}

func parseIPv4(s string) []byte {
	var a, b, c, d int
	_, err := fmt.Sscanf(s, "%d.%d.%d.%d", &a, &b, &c, &d)
	if err != nil {
		log.Printf("sscanf faile: %v", err)
	}
	return []byte{byte(a), byte(b), byte(c), byte(d)}
}
