package network

import (
	"fmt"
	"net"
)

func AddrKey(addr *net.UDPAddr) string {
	return fmt.Sprintf("%s:%d", addr.IP.String(), addr.Port)
}

const MulticastAddress = "239.192.0.4:9192"
