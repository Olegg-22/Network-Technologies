package network

import (
	"net"
	"time"

	"lab4/internal/domain"
)

type Peer struct {
	ID           int32
	Name         string
	Addr         *net.UDPAddr
	Role         domain.NodeRole
	LastReceived time.Time
	LastSent     time.Time
}

func NewPeer(id int32, name string, addr *net.UDPAddr, role domain.NodeRole) *Peer {
	now := time.Now()
	return &Peer{
		ID:           id,
		Name:         name,
		Addr:         addr,
		Role:         role,
		LastReceived: now,
		LastSent:     now,
	}
}

func (p *Peer) UpdateLastReceived() {
	p.LastReceived = time.Now()
}

func (p *Peer) UpdateLastSent() {
	p.LastSent = time.Now()
}

func (p *Peer) IsTimedOut(timeout time.Duration) bool {
	return time.Since(p.LastReceived) > timeout
}

func (p *Peer) NeedsPing(interval time.Duration) bool {
	return time.Since(p.LastSent) > interval
}
