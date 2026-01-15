package network

import (
	"log"
	"net"
	"strings"
	"sync/atomic"
	"time"

	pb "lab4/pkg/pb"

	"google.golang.org/protobuf/proto"
)

type Socket struct {
	conn          *net.UDPConn
	localAddr     *net.UDPAddr
	msgSeq        int64
	multicastAddr *net.UDPAddr
}

func NewSocket() (*Socket, error) {
	bindIP := findBestBindIP()

	addr := &net.UDPAddr{
		IP:   bindIP,
		Port: 0,
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}

	multicastAddr, _ := net.ResolveUDPAddr("udp4", MulticastAddress)

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	log.Printf("Socket: listening on %s (bound to %s)", localAddr, bindIP)

	return &Socket{
		conn:          conn,
		localAddr:     localAddr,
		msgSeq:        0,
		multicastAddr: multicastAddr,
	}, nil
}

func findBestBindIP() net.IP {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Failed to get interfaces, using 0.0.0.0")
		return net.IPv4zero
	}

	var bestIP net.IP
	var fallbackIP net.IP

	for _, ifi := range interfaces {
		if ifi.Flags&net.FlagUp == 0 {
			continue
		}
		if ifi.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				ip := ipnet.IP.To4()
				if ip == nil || ip.IsLoopback() {
					continue
				}

				name := strings.ToLower(ifi.Name)

				if strings.Contains(name, "wsl") ||
					strings.Contains(name, "hyper-v") ||
					strings.Contains(name, "virtual") ||
					strings.Contains(name, "vethernet") ||
					strings.Contains(name, "vmware") ||
					strings.Contains(name, "docker") {
					continue
				}

				if ip[0] == 192 && ip[1] == 168 && ip[2] == 56 {
					continue
				}

				if strings.Contains(name, "wifi") ||
					strings.Contains(name, "wireless") ||
					strings.Contains(name, "wlan") ||
					strings.Contains(name, "беспроводн") ||
					strings.Contains(name, "ethernet") ||
					strings.Contains(name, "eth") {
					log.Printf("Found preferred interface: %s with IP %s", ifi.Name, ip)
					bestIP = ip
					break
				}

				if fallbackIP == nil {
					log.Printf("Found fallback interface: %s with IP %s", ifi.Name, ip)
					fallbackIP = ip
				}
			}
		}

		if bestIP != nil {
			break
		}
	}

	if bestIP != nil {
		log.Printf("Selected bind IP: %s", bestIP)
		return bestIP
	}
	if fallbackIP != nil {
		log.Printf("Using fallback bind IP: %s", fallbackIP)
		return fallbackIP
	}

	log.Printf("No good interface found, using 0.0.0.0")
	return net.IPv4zero
}

func (s *Socket) Send(msg *pb.GameMessage, addr *net.UDPAddr) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = s.conn.WriteToUDP(data, addr)
	return err
}

func (s *Socket) SendMulticast(msg *pb.GameMessage) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = s.conn.WriteToUDP(data, s.multicastAddr)

	broadcastAddr, _ := net.ResolveUDPAddr("udp4", "255.255.255.255:9192")
	s.conn.WriteToUDP(data, broadcastAddr)

	if s.localAddr.IP != nil && !s.localAddr.IP.IsUnspecified() {
		subnetBroadcast := make(net.IP, 4)
		copy(subnetBroadcast, s.localAddr.IP.To4())
		subnetBroadcast[3] = 255
		subnetAddr, _ := net.ResolveUDPAddr("udp4", subnetBroadcast.String()+":9192")
		s.conn.WriteToUDP(data, subnetAddr)
	}

	return err
}

func (s *Socket) Receive() (*pb.GameMessage, *net.UDPAddr, error) {
	buf := make([]byte, 65535)

	n, addr, err := s.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, nil, err
	}

	msg := &pb.GameMessage{}
	if err := proto.Unmarshal(buf[:n], msg); err != nil {
		return nil, nil, err
	}

	return msg, addr, nil
}

func (s *Socket) ReceiveWithTimeout(timeout time.Duration) (*pb.GameMessage, *net.UDPAddr, error) {
	s.conn.SetReadDeadline(time.Now().Add(timeout))
	defer s.conn.SetReadDeadline(time.Time{})

	return s.Receive()
}

func (s *Socket) NextSeq() int64 {
	return atomic.AddInt64(&s.msgSeq, 1)
}

func (s *Socket) LocalPort() int {
	return s.localAddr.Port
}

func (s *Socket) Close() error {
	return s.conn.Close()
}
