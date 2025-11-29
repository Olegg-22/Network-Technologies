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

func TryProcessHandshake(conn *data.Conn) {
	for {
		switch conn.State {
		case data.StateGreeting:
			if conn.HandshakeBuffer.Len() < 2 {
				return
			}
			b := conn.HandshakeBuffer.Bytes()
			if b[0] != data.SocksVer {
				utils.CloseConn(conn)
				return
			}
			nm := int(b[1])
			if conn.HandshakeBuffer.Len() < 2+nm {
				return
			}
			methods := b[2 : 2+nm]
			ok := false
			for _, m := range methods {
				if m == data.SocksMethodNoAuth {
					ok = true
					break
				}
			}
			conn.HandshakeBuffer.Next(2 + nm)

			if !utils.WriteAll(conn.ClientFD, []byte{data.SocksVer, data.SocksMethodNoAuth}) {
				utils.CloseConn(conn)
				return
			}
			if !ok {
				utils.CloseConn(conn)
				return
			}
			conn.State = data.StateRequest
		case data.StateRequest:
			if conn.HandshakeBuffer.Len() < 4 {
				return
			}
			b := conn.HandshakeBuffer.Bytes()
			if b[0] != data.SocksVer {
				utils.CloseConn(conn)
				return
			}
			cmd := b[1]
			atyp := b[3]
			if cmd != data.SocksCmdConnect {
				utils.SendSocksReply(conn.ClientFD, data.RepCommandNotSupported, atyp, nil, 0)
				utils.CloseConn(conn)
				return
			}
			if atyp == data.AtypIPv4 {
				if conn.HandshakeBuffer.Len() < 10 {
					return
				}
				addr := fmt.Sprintf("%d.%d.%d.%d", b[4], b[5], b[6], b[7])
				port := int(binary.BigEndian.Uint16(b[8:10]))
				conn.HandshakeBuffer.Next(10)
				if !connect.StartUpstreamConnect(conn, addr, port, false) {
					utils.CloseConn(conn)
					return
				}
				conn.State = data.StateConnecting
				return
			}

			if atyp == data.AtypDomain {
				if conn.HandshakeBuffer.Len() < 5 {
					return
				}
				dlen := int(b[4])
				if conn.HandshakeBuffer.Len() < 5+dlen+2 {
					return
				}
				domain := string(b[5 : 5+dlen])
				port := int(binary.BigEndian.Uint16(b[5+dlen : 5+dlen+2]))

				conn.HandshakeBuffer.Next(5 + dlen + 2)

				pr := &dns.PendingResolve{Conn: conn, Domain: domain, Port: port}
				_, err := dns.SendDNSQuery(domain, pr)
				if err != nil {
					utils.SendSocksReply(conn.ClientFD, data.RepGeneralFailure, data.AtypDomain, nil, 0)
					utils.CloseConn(conn)
					return
				}
				conn.State = data.StateResolving
				return
			}

			if atyp == data.AtypIPv6 {
				if conn.HandshakeBuffer.Len() < 4+16+2 {
					return
				}
				addrBytes := b[4 : 4+16]
				port := int(binary.BigEndian.Uint16(b[4+16 : 4+16+2]))
				conn.HandshakeBuffer.Next(4 + 16 + 2)
				addr := net.IP(addrBytes).String()
				if !connect.StartUpstreamConnect(conn, addr, port, true) {
					utils.CloseConn(conn)
					return
				}
				conn.State = data.StateConnecting
				return
			}

			utils.SendSocksReply(conn.ClientFD, data.RepAddrTypeNotSupported, atyp, nil, 0)
			utils.CloseConn(conn)
			return
		default:
			return
		}
	}
}
