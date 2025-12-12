package handshake

import (
	"encoding/binary"
	"fmt"
	"lab5/internal/connect"
	"lab5/internal/data"
	"lab5/internal/dns"
	"lab5/internal/utils"
	"net"
)

const (
	greetingHeaderSize = 2
	requestMinSize     = 4
	ipv4AddrSize       = 4
	ipv6AddrSize       = 16
	portSize           = 2
	ipv4RequestSize    = 10

	versionOffset      = 0
	methodsCountOffset = 1
	methodsStartOffset = 2
	commandOffset      = 1
	addressTypeOffset  = 3
	domainLenOffset    = 4
	domainStartOffset  = 5
)

func TryProcessHandshake(conn *data.Conn) {
	for {
		switch conn.State {
		case data.StateGreeting:
			if conn.HandshakeBuffer.Len() < greetingHeaderSize {
				return
			}

			handshakeBuffer := conn.HandshakeBuffer.Bytes()
			if handshakeBuffer[versionOffset] != data.SocksVer {
				utils.CloseConn(conn)
				return
			}

			methodsCount := int(handshakeBuffer[methodsCountOffset])

			if conn.HandshakeBuffer.Len() < greetingHeaderSize+methodsCount {
				return
			}

			methods := handshakeBuffer[methodsStartOffset : methodsStartOffset+methodsCount]

			noAuthSupported := false
			for _, method := range methods {
				if method == data.SocksMethodNoAuth {
					noAuthSupported = true
					break
				}
			}
			conn.HandshakeBuffer.Next(greetingHeaderSize + methodsCount)

			if !utils.WriteAll(conn, conn.ClientFD, []byte{data.SocksVer, data.SocksMethodNoAuth}, false) {
				utils.CloseConn(conn)
				return
			}

			if !noAuthSupported {
				utils.CloseConn(conn)
				return
			}

			conn.State = data.StateRequest

		case data.StateRequest:
			if conn.HandshakeBuffer.Len() < requestMinSize {
				return
			}

			handshakeBuffer := conn.HandshakeBuffer.Bytes()
			if handshakeBuffer[versionOffset] != data.SocksVer {
				utils.CloseConn(conn)
				return
			}

			command := handshakeBuffer[commandOffset]
			addressType := handshakeBuffer[addressTypeOffset]

			if command != data.SocksCmdConnect {
				utils.SendSocksReply(conn, data.RepCommandNotSupported, addressType, nil, 0)
				utils.CloseConn(conn)
				return
			}
			if addressType == data.AtypIPv4 {
				if conn.HandshakeBuffer.Len() < ipv4RequestSize {
					return
				}

				ipStart := addressTypeOffset + 1
				ipEnd := ipStart + ipv4AddrSize
				portStart := ipEnd
				portEnd := portStart + portSize

				addr := fmt.Sprintf("%d.%d.%d.%d",
					handshakeBuffer[ipStart],
					handshakeBuffer[ipStart+1],
					handshakeBuffer[ipStart+2],
					handshakeBuffer[ipStart+3])

				port := int(binary.BigEndian.Uint16(handshakeBuffer[portStart:portEnd]))

				conn.HandshakeBuffer.Next(ipv4RequestSize)
				if !connect.StartUpstreamConnect(conn, addr, port, false) {
					utils.CloseConn(conn)
					return
				}

				conn.State = data.StateConnecting
				return
			}

			if addressType == data.AtypDomain {
				domainMinSize := requestMinSize + 1
				if conn.HandshakeBuffer.Len() < domainMinSize {
					return
				}

				domainLen := int(handshakeBuffer[domainLenOffset])
				if conn.HandshakeBuffer.Len() < domainMinSize+domainLen+portSize {
					return
				}

				domain := string(handshakeBuffer[domainStartOffset : domainStartOffset+domainLen])

				portStart := domainStartOffset + domainLen
				portEnd := portStart + portSize
				port := int(binary.BigEndian.Uint16(handshakeBuffer[portStart:portEnd]))

				conn.HandshakeBuffer.Next(domainMinSize + domainLen + portSize)

				pr := &dns.PendingResolve{Conn: conn, Domain: domain, Port: port}
				_, err := dns.SendDNSQuery(domain, pr)
				if err != nil {
					utils.SendSocksReply(conn, data.RepGeneralFailure, data.AtypDomain, nil, 0)
					utils.CloseConn(conn)
					return
				}
				conn.State = data.StateResolving
				return
			}

			if addressType == data.AtypIPv6 {
				ipv6RequestSize := requestMinSize + ipv6AddrSize + portSize
				if conn.HandshakeBuffer.Len() < ipv6RequestSize {
					return
				}

				ipStart := addressTypeOffset + 1
				ipEnd := ipStart + ipv6AddrSize
				portStart := ipEnd
				portEnd := portStart + portSize

				addrBytes := handshakeBuffer[ipStart:ipEnd]
				port := int(binary.BigEndian.Uint16(handshakeBuffer[portStart:portEnd]))

				conn.HandshakeBuffer.Next(ipv6RequestSize)
				addr := net.IP(addrBytes).String()
				if !connect.StartUpstreamConnect(conn, addr, port, true) {
					utils.CloseConn(conn)
					return
				}
				conn.State = data.StateConnecting
				return
			}

			utils.SendSocksReply(conn, data.RepAddrTypeNotSupported, addressType, nil, 0)
			utils.CloseConn(conn)
			return
		default:
			return
		}
	}
}
