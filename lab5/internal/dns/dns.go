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
	IsIPv6 bool
}

var (
	FD              int = -1
	pendingResolves     = make(map[uint16]*PendingResolve)
	dnsResolverAddr     = &unix.SockaddrInet4{Port: 53, Addr: [4]byte{8, 8, 8, 8}}
)

const (
	flags              uint16 = 0x0100 // QR=0, OPCODE=0, RD=1
	questionsCount     uint16 = 1
	answersCount       uint16 = 0
	authorityRRsCount  uint16 = 0 // Authority Resource Records
	additionalRRsCount uint16 = 0 // Additional Resource Records

	limitCharInLabelLen = 63

	dnsTypeA    uint16 = 1
	dnsTypeAAAA uint16 = 28

	dnsClassIN uint16 = 1

	dnsHeaderSize    = 12
	dnsIDOffset      = 0
	dnsFlagsOffset   = 2
	dnsQDCountOffset = 4
	dnsANCountOffset = 6

	dnsQRMask    = 0x8000
	dnsRcodeMask = 0x000F

	dnsPointerMask = 0xC0

	dnsAnswerMinSize = 10
	dnsTypeClassSize = 4
	dnsTTLSize       = 4
	dnsIPv4Size      = 4
	dnsIPv6Size      = 16

	dnsQRResponse = 1

	maxDnsID                     = 0xFFFF
	maxRetryAttemptsForFindDnsID = 10

	dnsBufferSize = 4 * 1024
)

func buildDNSQuery(id uint16, name string, isIPv6 bool) ([]byte, error) {
	dnsQuery := &bytes.Buffer{}
	_ = binary.Write(dnsQuery, binary.BigEndian, id)
	_ = binary.Write(dnsQuery, binary.BigEndian, flags)
	_ = binary.Write(dnsQuery, binary.BigEndian, questionsCount)
	_ = binary.Write(dnsQuery, binary.BigEndian, answersCount)
	_ = binary.Write(dnsQuery, binary.BigEndian, authorityRRsCount)
	_ = binary.Write(dnsQuery, binary.BigEndian, additionalRRsCount)

	for _, label := range bytes.Split([]byte(name), []byte{'.'}) {
		if len(label) == 0 || len(label) > limitCharInLabelLen {
			return nil, fmt.Errorf("invalid dns name")
		}
		dnsQuery.WriteByte(byte(len(label)))
		dnsQuery.Write(label)
	}
	dnsQuery.WriteByte(0)
	var queryType uint16
	if isIPv6 {
		queryType = dnsTypeAAAA
	} else {
		queryType = dnsTypeA
	}
	_ = binary.Write(dnsQuery, binary.BigEndian, queryType)
	_ = binary.Write(dnsQuery, binary.BigEndian, dnsClassIN)
	return dnsQuery.Bytes(), nil
}

func parseDNSResponse(dnsResponse []byte, isIPv6 bool) (uint16, string, error) {
	if len(dnsResponse) < dnsHeaderSize {
		return 0, "", fmt.Errorf("short dns response")
	}

	id := binary.BigEndian.Uint16(dnsResponse[dnsIDOffset : dnsIDOffset+2])
	flags := binary.BigEndian.Uint16(dnsResponse[dnsFlagsOffset : dnsFlagsOffset+2])

	if (flags&dnsQRMask)>>15 != dnsQRResponse {
		return id, "", fmt.Errorf("not a response")
	}

	rcode := flags & dnsRcodeMask
	if rcode != 0 {
		return id, "", fmt.Errorf("rcode=%d", rcode)
	}

	qdcount := int(binary.BigEndian.Uint16(dnsResponse[dnsQDCountOffset : dnsQDCountOffset+2]))
	ancount := int(binary.BigEndian.Uint16(dnsResponse[dnsANCountOffset : dnsANCountOffset+2]))

	offset := dnsHeaderSize
	for i := 0; i < qdcount; i++ {
		for {
			if offset >= len(dnsResponse) {
				return id, "", fmt.Errorf("uncorrect response")
			}
			labelLength := int(dnsResponse[offset])
			offset++
			if labelLength == 0 {
				break
			}
			offset += labelLength
		}
		offset += dnsTypeClassSize
	}

	var expectedType uint16
	var expectedSize int
	if isIPv6 {
		expectedType = dnsTypeAAAA
		expectedSize = dnsIPv6Size
	} else {
		expectedType = dnsTypeA
		expectedSize = dnsIPv4Size
	}

	for i := 0; i < ancount; i++ {
		if offset+dnsAnswerMinSize > len(dnsResponse) {
			return id, "", fmt.Errorf("short answer")
		}
		if dnsResponse[offset]&dnsPointerMask == dnsPointerMask {
			offset += 2
		} else {
			for {
				if offset >= len(dnsResponse) {
					return id, "", fmt.Errorf("uncorrect answer name")
				}
				labelLength := int(dnsResponse[offset])
				offset++
				if labelLength == 0 {
					break
				}
				offset += labelLength
			}
		}
		typ := binary.BigEndian.Uint16(dnsResponse[offset : offset+2])
		offset += 2

		class := binary.BigEndian.Uint16(dnsResponse[offset : offset+2])
		offset += 2

		offset += dnsTTLSize

		rdlen := int(binary.BigEndian.Uint16(dnsResponse[offset : offset+2]))
		offset += 2

		if offset+rdlen > len(dnsResponse) {
			return id, "", fmt.Errorf("rdata out of bounds")
		}
		rdata := dnsResponse[offset : offset+rdlen]
		offset += rdlen

		if typ == expectedType && class == dnsClassIN && rdlen == expectedSize {
			var ip string
			if isIPv6 {
				if len(rdata) == dnsIPv6Size {
					ip = net.IP(rdata).String()
				}
			} else {
				if len(rdata) == dnsIPv4Size {
					ip = net.IPv4(rdata[0], rdata[1], rdata[2], rdata[3]).String()
				}
			}

			if ip != "" {
				return id, ip, nil
			}
		}
	}
	return id, "", fmt.Errorf("no record found")
}

func SendDNSQuery(domain string, p *PendingResolve) (uint16, error) {
	var id uint16
	for tries := 0; tries < maxRetryAttemptsForFindDnsID; tries++ {
		id = uint16(rand.Intn(maxDnsID))
		if _, exists := pendingResolves[id]; !exists {
			break
		}
		if tries == maxRetryAttemptsForFindDnsID-1 {
			return 0, fmt.Errorf("faile to find free dns id")
		}
	}

	dnsQuery, err := buildDNSQuery(id, domain, p.IsIPv6)
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
	dnsBuffer := make([]byte, dnsBufferSize)
	for {
		n, _, err := unix.Recvfrom(FD, dnsBuffer, 0)
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

		id, ipStr, err := parseDNSResponse(dnsBuffer[:n], false)
		if err != nil {
			pendingRequest := pendingResolves[id]
			if pendingRequest != nil {
				id, ipStr, err = parseDNSResponse(dnsBuffer[:n], pendingRequest.IsIPv6)
			}

			if err != nil {
				fmt.Printf("dns parse err: %v\n", err)
				continue
			}
		}

		pendingRequest := pendingResolves[id]
		if pendingRequest == nil {
			continue
		}
		delete(pendingResolves, id)

		ip := net.ParseIP(ipStr)
		if ip == nil {
			utils.SendSocksReply(pendingRequest.Conn, data.RepGeneralFailure, data.AtypDomain, nil, 0)
			utils.CloseConn(pendingRequest.Conn)
			continue
		}
		isIPv6 := pendingRequest.IsIPv6

		if !connect.StartUpstreamConnect(pendingRequest.Conn, ipStr, pendingRequest.Port, isIPv6) {
			utils.CloseConn(pendingRequest.Conn)
			continue
		}
		pendingRequest.Conn.State = data.StateConnecting
	}
}
