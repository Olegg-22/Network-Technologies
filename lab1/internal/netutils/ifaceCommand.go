package netutils

import (
	"net"
)

const (
	localhostIPv4 = "127.0.0.1"
	localhostIPv6 = "::1"
)

func IfaceValidIPs(iface *net.Interface, useIPv6 bool) []net.IP {
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

func IfaceLocalIP(iface *net.Interface, useIPv6 bool) net.IP {
	ips := IfaceValidIPs(iface, useIPv6)
	if len(ips) > 0 {
		return ips[0]
	}
	if useIPv6 {
		return net.ParseIP(localhostIPv6)
	}
	return net.ParseIP(localhostIPv4)
}
