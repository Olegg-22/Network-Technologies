package dns

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"lab5/internal/connect"
	"lab5/internal/data"
	"lab5/internal/utils"
	"math/rand"
	"net"

	"golang.org/x/sys/unix"
)

type PendingResolve struct {
	Conn   *data.Conn
	Domain string
	Port   int
}

var (
	FD              int = -1
	pendingResolves     = make(map[uint16]*PendingResolve)
	dnsResolverAddr     = &unix.SockaddrInet4{Port: 53, Addr: [4]byte{8, 8, 8, 8}}
)

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

func SendDNSQuery(domain string, p *PendingResolve) (uint16, error) {
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

	if err := unix.Sendto(FD, dnsQuery, 0, dnsResolverAddr); err != nil {
		return 0, err
	}

	pendingResolves[id] = p
	return id, nil
}

func HandleDNSRead() {
	buf := make([]byte, 4*1024)
	for {
		n, _, err := unix.Recvfrom(FD, buf, 0)
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
			//fmt.Printf("dns parse err: %v\n", err)
			continue
		}

		pendingRequest := pendingResolves[id]
		if pendingRequest == nil {
			continue
		}
		delete(pendingResolves, id)

		ip := net.ParseIP(ipStr)
		if ip == nil {
			utils.SendSocksReply(pendingRequest.Conn.ClientFD, data.RepGeneralFailure, data.AtypDomain, nil, 0)
			utils.CloseConn(pendingRequest.Conn)
			continue
		}
		isIPv6 := false
		if ip.To4() == nil {
			isIPv6 = true
		}

		if !connect.StartUpstreamConnect(pendingRequest.Conn, ipStr, pendingRequest.Port, isIPv6) {
			utils.CloseConn(pendingRequest.Conn)
			continue
		}
		pendingRequest.Conn.State = data.StateConnecting
	}
}
