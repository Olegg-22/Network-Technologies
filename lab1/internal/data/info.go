package data

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"lab1/internal/netutils"
	"lab1/internal/utils"
)

const (
	SizeBuffer            = 128
	CountReceivedArgument = 2
)

type Connection struct {
	ID       string
	Addr     string
	LastSeen time.Time
}

type InfoStruct struct {
	group       *net.UDPAddr
	RecvConn    *net.UDPConn
	SendConn    *net.UDPConn
	Id          string
	SelfAddr    string
	Connections map[string]Connection
	Mu          sync.Mutex
	PrintCh     chan struct{}
	Ctx         context.Context
	Cancel      context.CancelFunc
	Wg          sync.WaitGroup
}

func NewInfoStruct(group string, preferredIface string) (*InfoStruct, error) {
	gaddr, err := net.ResolveUDPAddr("udp", group)
	if err != nil {
		return nil, fmt.Errorf("ResolveUDPAddr error: %w", err)
	}

	useIPv6 := gaddr.IP.To4() == nil
	network := "udp4"
	if useIPv6 {
		network = "udp6"
	}

	hasAddrFor := func(i *net.Interface) bool {
		return len(netutils.IfaceValidIPs(i, useIPv6)) > 0
	}

	var iface *net.Interface
	if preferredIface != "" {
		i, err := net.InterfaceByName(preferredIface)
		if err != nil {
			return nil, fmt.Errorf("interface %s not found: %w", preferredIface, err)
		}
		if (i.Flags&net.FlagUp) == 0 || (i.Flags&net.FlagMulticast) == 0 {
			return nil, fmt.Errorf("interface %s is not up or doesn't support multicast", preferredIface)
		}
		if !hasAddrFor(i) {
			return nil, fmt.Errorf("interface %s does not have an address for the required IPs", preferredIface)
		}
		iface = i
	} else {
		ifaces, _ := net.Interfaces()
		for _, i := range ifaces {
			if len(netutils.IfaceValidIPs(&i, useIPv6)) > 0 {
				iface = &i
				if useIPv6 {
					gaddr.Zone = i.Name
				}
				break
			}
		}
	}
	if iface == nil {
		return nil, fmt.Errorf("no multicast interfaces available")
	}

	recv, err := net.ListenMulticastUDP(network, iface, gaddr)
	if err != nil {
		return nil, fmt.Errorf("listen multicast: %w", err)
	}

	localIP := netutils.IfaceLocalIP(iface, useIPv6)
	localAddr := &net.UDPAddr{IP: localIP, Port: 0}
	if useIPv6 && localIP.IsLinkLocalUnicast() {
		localAddr.Zone = iface.Name
	}
	send, err := net.DialUDP(network, localAddr, gaddr)
	if err != nil {
		return nil, fmt.Errorf("dial udp: %w", err)
	}

	id, err := utils.RandomID()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}
	selfIP := localIP.String()
	if useIPv6 && localIP.IsLinkLocalUnicast() {
		selfIP = localIP.String() + "%" + iface.Name
	}
	ctx, cancel := context.WithCancel(context.Background())

	return &InfoStruct{
		group:       gaddr,
		RecvConn:    recv,
		SendConn:    send,
		Id:          id,
		SelfAddr:    selfIP,
		Connections: make(map[string]Connection),
		PrintCh:     make(chan struct{}, 1),
		Ctx:         ctx,
		Cancel:      cancel,
	}, nil
}
