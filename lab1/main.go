package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	countByteId           = 4
	localhost             = "127.0.0.1"
	sizeBuffer            = 128
	countReceivedArgument = 2
)

type Connection struct {
	ID       string
	Addr     string
	LastSeen time.Time
}

type InfoStruct struct {
	group       *net.UDPAddr
	recvConn    *net.UDPConn
	sendConn    *net.UDPConn
	id          string
	selfAddr    string
	connections map[string]Connection
	mu          sync.Mutex
}

func newInfoStruct(group string) (*InfoStruct, error) {
	gaddr, err := net.ResolveUDPAddr("udp", group)
	if err != nil {
		return nil, err
	}
	network := "udp4"
	if gaddr.IP.To4() == nil {
		network = "udp6"
		if gaddr.Zone == "" {
			ifaces, _ := net.Interfaces()
			for _, iface := range ifaces {
				if (iface.Flags&net.FlagUp) != 0 && (iface.Flags&net.FlagMulticast) != 0 {
					gaddr.Zone = iface.Name
					break
				}
			}
		}
	}
	recv, err := net.ListenMulticastUDP(network, nil, gaddr)
	if err != nil {
		return nil, err
	}
	send, err := net.DialUDP(network, nil, gaddr)
	if err != nil {
		return nil, err
	}
	id := randomID()

	selfIP := getLocalIP()

	return &InfoStruct{
		group:       gaddr,
		recvConn:    recv,
		sendConn:    send,
		id:          id,
		selfAddr:    selfIP,
		connections: make(map[string]Connection),
	}, nil
}

func randomID() string {
	b := make([]byte, countByteId)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getLocalIP() string {
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return localhost
}

func (d *InfoStruct) run() {
	go d.sendConnect()
	go d.receive()
	go d.cleanup()
}

func (d *InfoStruct) sendConnect() {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for range t.C {
		msg := []byte("CONNECT:" + d.id)
		d.sendConn.Write(msg)
	}
}

func (d *InfoStruct) receive() {
	buf := make([]byte, sizeBuffer)
	for {
		n, src, err := d.recvConn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		msg := string(buf[:n])
		parts := strings.Split(msg, ":")
		if len(parts) != countReceivedArgument || parts[0] != "CONNECT" {
			continue
		}
		connectionID := parts[1]
		if connectionID == d.id {
			continue
		}
		d.mu.Lock()
		d.connections[connectionID] = Connection{ID: connectionID, Addr: src.IP.String(), LastSeen: time.Now()}
		d.printConnections()
		d.mu.Unlock()
	}
}

func (d *InfoStruct) cleanup() {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		d.mu.Lock()
		changed := false
		for id, p := range d.connections {
			if now.Sub(p.LastSeen) > 6*time.Second {
				delete(d.connections, id)
				changed = true
			}
		}
		if changed {
			d.printConnections()
		}
		d.mu.Unlock()
	}
}

func (d *InfoStruct) printConnections() {
	fmt.Println("===================================")
	fmt.Printf("🟢 Myself: %s (%s)\n", d.id, d.selfAddr)
	fmt.Println("Connections:")
	if len(d.connections) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, p := range d.connections {
			fmt.Printf("  🔸 %s (%s)\n", p.ID, p.Addr)
		}
	}
	fmt.Println("===================================")
}

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Println("Usage: go run main.go <multicastAddr:port>")
		return
	}
	group := flag.Arg(0)
	d, err := newInfoStruct(group)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	d.run()
	select {}
}
