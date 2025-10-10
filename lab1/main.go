package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	countByteId           = 4
	localhostIPv4         = "127.0.0.1"
	localhostIPv6         = "::1"
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
	printCh     chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func newInfoStruct(group string, preferredIface string) (*InfoStruct, error) {
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
		return len(ifaceValidIPs(i, useIPv6)) > 0
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
			if len(ifaceValidIPs(&i, useIPv6)) > 0 {
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

	localIP := ifaceLocalIP(iface, useIPv6)
	localAddr := &net.UDPAddr{IP: localIP, Port: 0}
	if useIPv6 && localIP.IsLinkLocalUnicast() {
		localAddr.Zone = iface.Name
	}
	send, err := net.DialUDP(network, localAddr, gaddr)
	if err != nil {
		return nil, fmt.Errorf("dial udp: %w", err)
	}

	id, err := randomID()
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
		recvConn:    recv,
		sendConn:    send,
		id:          id,
		selfAddr:    selfIP,
		connections: make(map[string]Connection),
		printCh:     make(chan struct{}, 1),
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

func randomID() (string, error) {
	b := make([]byte, countByteId)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), err
}

func ifaceValidIPs(iface *net.Interface, useIPv6 bool) []net.IP {
	addrs, _ := iface.Addrs()
	var result []net.IP
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			if useIPv6 {
				if ipnet.IP.To4() == nil && !ipnet.IP.IsLoopback() {
					result = append(result, ipnet.IP)
				}
			} else {
				if ip := ipnet.IP.To4(); ip != nil && !ip.IsLoopback() {
					result = append(result, ip)
				}
			}
		}
	}
	return result
}

func ifaceLocalIP(iface *net.Interface, useIPv6 bool) net.IP {
	ips := ifaceValidIPs(iface, useIPv6)
	if len(ips) > 0 {
		return ips[0]
	}
	if useIPv6 {
		return net.ParseIP(localhostIPv6)
	}
	return net.ParseIP(localhostIPv4)
}

func (lib *InfoStruct) run() {
	lib.wg.Add(1)
	go func() {
		defer lib.wg.Done()
		lib.sendConnect(lib.ctx)
	}()

	lib.wg.Add(1)
	go func() {
		defer lib.wg.Done()
		lib.receive(lib.ctx)
	}()

	lib.wg.Add(1)
	go func() {
		defer lib.wg.Done()
		lib.cleanup(lib.ctx)
	}()

	lib.wg.Add(1)
	go func() {
		defer lib.wg.Done()
		lib.printConnections(lib.ctx)
	}()
}

func (lib *InfoStruct) sendConnect(ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			msg := []byte("CONNECT:" + lib.id)
			if lib.sendConn == nil {
				continue
			}
			if _, err := lib.sendConn.Write(msg); err != nil {
				select {
				case <-ctx.Done():
					return
				default:
				}
				fmt.Println("send error:", err)
			}
		}
	}
}

func (lib *InfoStruct) receive(ctx context.Context) {
	buf := make([]byte, sizeBuffer)
	for {
		_ = lib.recvConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, src, err := lib.recvConn.ReadFromUDP(buf)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			fmt.Println("read error:", err)
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
		lib.mu.Unlock()

		select {
		case lib.printCh <- struct{}{}:
		default:
		}
	}
}

func (lib *InfoStruct) cleanup(ctx context.Context) {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now()
			changed := false
			lib.mu.Lock()
			for id, p := range lib.connections {
				if now.Sub(p.LastSeen) > 6*time.Second {
					delete(lib.connections, id)
					changed = true
				}
			}
			lib.mu.Unlock()

			if changed {
				select {
				case lib.printCh <- struct{}{}:
				default:
				}
			}
		}
	}
}

func (lib *InfoStruct) printConnections(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-lib.printCh:
			lib.mu.Lock()
			id := lib.id
			selfAddr := lib.selfAddr
			conns := make([]Connection, 0, len(lib.connections))
			for _, c := range lib.connections {
				conns = append(conns, c)
			}
			lib.mu.Unlock()

			fmt.Println("===================================")
			fmt.Printf("🟢 Myself: %s (%s)\n", id, selfAddr)
			fmt.Println("Connections:")
			if len(conns) == 0 {
				fmt.Println("  (none)")
			} else {
				for _, p := range conns {
					fmt.Printf("  🔸 %s (%s)\n", p.ID, p.Addr)
				}
			}
			fmt.Println("===================================")
		}
	}
}

func (lib *InfoStruct) Shutdown() {
	lib.cancel()

	if lib.recvConn != nil {
		_ = lib.recvConn.Close()
	}
	if lib.sendConn != nil {
		_ = lib.sendConn.Close()
	}

	lib.wg.Wait()
}

var ifaceName = flag.String("iface", "", "network interface to use (optional)")

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Println("Usage: go run main.go [-iface en0] <multicastAddr:port>")
		return
	}
	group := flag.Arg(0)
	lib, err := newInfoStruct(group, *ifaceName)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	lib.run()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	fmt.Println("\n===SHUTDOWN...===")
	lib.Shutdown()
	fmt.Println("\n===SHUTDOWN END===")
}
