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
	}
	var iface *net.Interface
	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		if (i.Flags&net.FlagUp) != 0 && (i.Flags&net.FlagMulticast) != 0 {
			iface = &i
			break
		}
	}
	if iface == nil {
		return nil, fmt.Errorf("no multicast interfaces available")
	}

	recv, err := net.ListenMulticastUDP(network, iface, gaddr)
	if err != nil {
		return nil, fmt.Errorf("listen multicast: %w", err)
	}

	localAddr := &net.UDPAddr{IP: ifaceLocalIP(iface), Port: 0}
	send, err := net.DialUDP(network, localAddr, gaddr)
	if err != nil {
		return nil, fmt.Errorf("dial udp: %w", err)
	}

	id := randomID()
	selfIP := ifaceLocalIP(iface).String()

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

func ifaceLocalIP(iface *net.Interface) net.IP {
	addrs, _ := iface.Addrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			return ipnet.IP
		}
	}
	return net.ParseIP(localhost)
}

func (lib *InfoStruct) run() {
	go lib.sendConnect()
	go lib.receive()
	go lib.cleanup()
}

func (lib *InfoStruct) sendConnect() {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for range t.C {
		msg := []byte("CONNECT:" + lib.id)
		lib.sendConn.Write(msg)
	}
}

func (lib *InfoStruct) receive() {
	buf := make([]byte, sizeBuffer)
	for {
		n, src, err := lib.recvConn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		msg := string(buf[:n])
		parts := strings.Split(msg, ":")
		if len(parts) != countReceivedArgument || parts[0] != "CONNECT" {
			continue
		}
		connectionID := parts[1]
		if connectionID == lib.id {
			continue
		}
		lib.mu.Lock()
		lib.connections[connectionID] = Connection{ID: connectionID, Addr: src.IP.String(), LastSeen: time.Now()}
		lib.printConnections()
		lib.mu.Unlock()
	}
}

func (lib *InfoStruct) cleanup() {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		lib.mu.Lock()
		changed := false
		for id, p := range lib.connections {
			if now.Sub(p.LastSeen) > 6*time.Second {
				delete(lib.connections, id)
				changed = true
			}
		}
		if changed {
			lib.printConnections()
		}
		lib.mu.Unlock()
	}
}

func (lib *InfoStruct) printConnections() {
	fmt.Println("===================================")
	fmt.Printf("🟢 Myself: %s (%s)\n", lib.id, lib.selfAddr)
	fmt.Println("Connections:")
	if len(lib.connections) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, p := range lib.connections {
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
	lib, err := newInfoStruct(group)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	lib.run()
	select {}
}
