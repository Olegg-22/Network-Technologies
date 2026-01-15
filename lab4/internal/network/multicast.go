package network

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	pb "lab4/pkg/pb"

	"google.golang.org/protobuf/proto"
)

const MulticastPort = 9192

type Multicast struct {
	recvConn *net.UDPConn
	addr     *net.UDPAddr
}

func NewMulticast() (*Multicast, error) {
	multicastAddr, err := net.ResolveUDPAddr("udp4", MulticastAddress)
	if err != nil {
		return nil, fmt.Errorf("resolve multicast addr: %w", err)
	}

	var recvConn *net.UDPConn

	ifi, err := findBestInterface()
	if err == nil && ifi != nil {
		log.Printf("Multicast: trying interface %s", ifi.Name)

		recvConn, err = net.ListenMulticastUDP("udp4", ifi, multicastAddr)
		if err == nil {
			log.Printf("Multicast: listening on interface %s", ifi.Name)
		}
	}

	if recvConn == nil {
		log.Println("Multicast: trying without specific interface")

		recvConn, err = net.ListenMulticastUDP("udp4", nil, multicastAddr)
		if err != nil {
			log.Println("Multicast: fallback to regular UDP")
			listenAddr := &net.UDPAddr{
				IP:   net.IPv4zero,
				Port: MulticastPort,
			}
			recvConn, err = net.ListenUDP("udp4", listenAddr)
			if err != nil {
				return nil, fmt.Errorf("all multicast methods failed: %w", err)
			}
		}
	}

	recvConn.SetReadBuffer(65535)

	log.Printf("Multicast: ready, local addr = %s", recvConn.LocalAddr())

	return &Multicast{
		recvConn: recvConn,
		addr:     multicastAddr,
	}, nil
}

func findBestInterface() (*net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var best *net.Interface
	var fallback *net.Interface

	for i := range interfaces {
		ifi := &interfaces[i]

		if ifi.Flags&net.FlagUp == 0 {
			continue
		}
		if ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		if ifi.Flags&net.FlagMulticast == 0 {
			continue
		}

		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				ip := ipnet.IP.To4()
				if ip != nil && !ip.IsLoopback() {
					name := strings.ToLower(ifi.Name)

					if strings.Contains(name, "wsl") ||
						strings.Contains(name, "hyper-v") ||
						strings.Contains(name, "virtual") ||
						strings.Contains(name, "vethernet") ||
						strings.Contains(name, "vmware") ||
						strings.Contains(name, "virtualbox") ||
						strings.Contains(name, "docker") {
						log.Printf("Skipping virtual interface: %s with IP %s", ifi.Name, ip)
						continue
					}

					if ip[0] == 192 && ip[1] == 168 && ip[2] == 56 {
						log.Printf("Skipping VirtualBox network: %s with IP %s", ifi.Name, ip)
						continue
					}

					if strings.Contains(name, "wifi") ||
						strings.Contains(name, "wireless") ||
						strings.Contains(name, "wlan") ||
						strings.Contains(name, "беспроводн") {
						log.Printf("Found WiFi interface: %s with IP %s", ifi.Name, ip)
						best = ifi
						break
					}

					if fallback == nil {
						log.Printf("Found interface: %s with IP %s", ifi.Name, ip)
						fallback = ifi
					}
				}
			}
		}

		if best != nil {
			break
		}
	}

	if best == nil {
		best = fallback
	}

	if best != nil {
		log.Printf("Selected interface: %s", best.Name)
	}

	return best, nil
}

func (m *Multicast) Receive() (*pb.GameMessage, *net.UDPAddr, error) {
	buf := make([]byte, 65535)

	n, addr, err := m.recvConn.ReadFromUDP(buf)
	if err != nil {
		return nil, nil, err
	}

	msg := &pb.GameMessage{}
	if err := proto.Unmarshal(buf[:n], msg); err != nil {
		return nil, nil, err
	}

	return msg, addr, nil
}

func (m *Multicast) ReceiveWithTimeout(timeout time.Duration) (*pb.GameMessage, *net.UDPAddr, error) {
	m.recvConn.SetReadDeadline(time.Now().Add(timeout))
	defer m.recvConn.SetReadDeadline(time.Time{})

	return m.Receive()
}

func (m *Multicast) Close() error {
	return m.recvConn.Close()
}

func (m *Multicast) Address() *net.UDPAddr {
	return m.addr
}
