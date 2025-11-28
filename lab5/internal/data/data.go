package data

import "bytes"

const (
	SocksVer = 0x05

	SocksMethodNoAuth = 0x00

	SocksCmdConnect = 0x01

	AtypIPv4   = 0x01
	AtypDomain = 0x03
	AtypIPv6   = 0x04

	RepSuccess              = 0x00
	RepGeneralFailure       = 0x01
	RepCommandNotSupported  = 0x07
	RepAddrTypeNotSupported = 0x08
)

const (
	StateGreeting   = 0
	StateRequest    = 1
	StateConnecting = 2
	StateRelaying   = 3
	StateResolving  = 4
)

type Conn struct {
	ClientFD   int
	UpstreamFD int

	HandshakeBuffer bytes.Buffer

	ClientToUpstreamBuffer bytes.Buffer
	UpstreamToClientBuffer bytes.Buffer

	State          int
	ClientClosed   bool
	UpstreamClosed bool
}

type FDInfo struct {
	Conn     *Conn
	IsClient bool
}

var (
	Epfd    int
	FdsInfo = make(map[int]*FDInfo)
	Conns   = make(map[int]*Conn)
)
