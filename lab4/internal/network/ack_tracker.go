package network

import (
	"net"
	"sync"
	"time"

	pb "lab4/pkg/pb"
)

type PendingMessage struct {
	Message  *pb.GameMessage
	Target   *net.UDPAddr
	SentAt   time.Time
	Attempts int
}

type AckTracker struct {
	pending       map[int64]*PendingMessage
	mu            sync.RWMutex
	retryInterval time.Duration
}

func NewAckTracker(retryInterval time.Duration) *AckTracker {
	return &AckTracker{
		pending:       make(map[int64]*PendingMessage),
		retryInterval: retryInterval,
	}
}

func (t *AckTracker) Track(seq int64, msg *pb.GameMessage, addr *net.UDPAddr) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pending[seq] = &PendingMessage{
		Message:  msg,
		Target:   addr,
		SentAt:   time.Now(),
		Attempts: 1,
	}
}

func (t *AckTracker) Acknowledge(seq int64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.pending[seq]; ok {
		delete(t.pending, seq)
		return true
	}
	return false
}

func (t *AckTracker) GetRetries() []*PendingMessage {
	t.mu.Lock()
	defer t.mu.Unlock()

	result := make([]*PendingMessage, 0)
	now := time.Now()

	for _, pm := range t.pending {
		if now.Sub(pm.SentAt) >= t.retryInterval {
			pm.SentAt = now
			pm.Attempts++
			result = append(result, pm)
		}
	}

	return result
}

func (t *AckTracker) RedirectTo(newAddr *net.UDPAddr) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, pm := range t.pending {
		pm.Target = newAddr
	}
}

func (t *AckTracker) RemoveByAddr(addr *net.UDPAddr) {
	t.mu.Lock()
	defer t.mu.Unlock()

	addrStr := addr.String()
	for seq, pm := range t.pending {
		if pm.Target.String() == addrStr {
			delete(t.pending, seq)
		}
	}
}
