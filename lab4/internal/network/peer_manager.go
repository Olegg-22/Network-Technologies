package network

import (
	"net"
	"sync"
	"time"

	"lab4/internal/domain"
)

type PeerManager struct {
	peers  map[int32]*Peer
	byAddr map[string]*Peer
	mu     sync.RWMutex
}

func NewPeerManager() *PeerManager {
	return &PeerManager{
		peers:  make(map[int32]*Peer),
		byAddr: make(map[string]*Peer),
	}
}

func (pm *PeerManager) Add(peer *Peer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.peers[peer.ID] = peer
	pm.byAddr[AddrKey(peer.Addr)] = peer
}

func (pm *PeerManager) Remove(id int32) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if peer, ok := pm.peers[id]; ok {
		delete(pm.byAddr, AddrKey(peer.Addr))
		delete(pm.peers, id)
	}
}

func (pm *PeerManager) Get(id int32) *Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.peers[id]
}

func (pm *PeerManager) GetByAddr(addr *net.UDPAddr) *Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.byAddr[AddrKey(addr)]
}

func (pm *PeerManager) GetAll() []*Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*Peer, 0, len(pm.peers))
	for _, peer := range pm.peers {
		result = append(result, peer)
	}
	return result
}

func (pm *PeerManager) FindTimedOut(timeout time.Duration) []*Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*Peer, 0)
	for _, peer := range pm.peers {
		if peer.IsTimedOut(timeout) {
			result = append(result, peer)
		}
	}
	return result
}

func (pm *PeerManager) FindNeedPing(interval time.Duration) []*Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*Peer, 0)
	for _, peer := range pm.peers {
		if peer.NeedsPing(interval) {
			result = append(result, peer)
		}
	}
	return result
}

func (pm *PeerManager) UpdateRole(id int32, role domain.NodeRole) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if peer, ok := pm.peers[id]; ok {
		peer.Role = role
	}
}

func (pm *PeerManager) Count() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.peers)
}

func (pm *PeerManager) UpdateAddr(id int32, addr *net.UDPAddr) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if peer, ok := pm.peers[id]; ok {
		delete(pm.byAddr, AddrKey(peer.Addr))
		peer.Addr = addr
		pm.byAddr[AddrKey(addr)] = peer
	}
}
