package app

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"lab4/internal/network"
	pb "lab4/pkg/pb"
)

type GameInfo struct {
	Name       string
	Players    int
	Config     *pb.GameConfig
	CanJoin    bool
	MasterAddr *net.UDPAddr
	LastSeen   time.Time
}

type DiscoveryEvent struct {
	Type    DiscoveryEventType
	Payload interface{}
}

type DiscoveryEventType int

const (
	DiscoveryGamesUpdated DiscoveryEventType = iota
)

type DiscoveryService struct {
	multicast *network.Multicast
	games     map[string]*GameInfo
	mu        sync.RWMutex
	eventCh   chan DiscoveryEvent
	cancel    context.CancelFunc
}

func NewDiscoveryService(multicast *network.Multicast) *DiscoveryService {
	return &DiscoveryService{
		multicast: multicast,
		games:     make(map[string]*GameInfo),
		eventCh:   make(chan DiscoveryEvent, 10),
	}
}

func (ds *DiscoveryService) Start(ctx context.Context) {
	ctx, ds.cancel = context.WithCancel(ctx)

	go ds.receiveLoop(ctx)
	go ds.cleanupLoop(ctx)
}

func (ds *DiscoveryService) Stop() {
	if ds.cancel != nil {
		ds.cancel()
	}
}

func (ds *DiscoveryService) Events() <-chan DiscoveryEvent {
	return ds.eventCh
}

func (ds *DiscoveryService) GetGames() []GameInfo {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	result := make([]GameInfo, 0, len(ds.games))
	for _, game := range ds.games {
		result = append(result, *game)
	}
	return result
}

func (ds *DiscoveryService) GetGame(name string) *GameInfo {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	if game, ok := ds.games[name]; ok {
		copy := *game
		return &copy
	}
	return nil
}

func (ds *DiscoveryService) receiveLoop(ctx context.Context) {
	log.Println("Discovery: receive loop started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Discovery: receive loop stopped")
			return
		default:
		}

		msg, addr, err := ds.multicast.ReceiveWithTimeout(500 * time.Millisecond)
		if err != nil {
			continue
		}

		//log.Printf("Discovery: received message from %s, type=%T", addr, msg.Type)

		if ann := msg.GetAnnouncement(); ann != nil {
			ds.handleAnnouncement(ann, addr)
		}
	}
}

func (ds *DiscoveryService) handleAnnouncement(ann *pb.GameMessage_AnnouncementMsg, addr *net.UDPAddr) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	updated := false

	for _, game := range ann.GetGames() {
		name := game.GetGameName()

		info := &GameInfo{
			Name:       name,
			Players:    len(game.GetPlayers().GetPlayers()),
			Config:     game.GetConfig(),
			CanJoin:    game.GetCanJoin(),
			MasterAddr: addr,
			LastSeen:   time.Now(),
		}

		if existing, ok := ds.games[name]; ok {
			if existing.Players != info.Players || existing.CanJoin != info.CanJoin {
				updated = true
			}
		} else {
			updated = true
			log.Printf("Discovered game: %s at %s", name, addr)
		}

		ds.games[name] = info
	}

	if updated {
		select {
		case ds.eventCh <- DiscoveryEvent{Type: DiscoveryGamesUpdated}:
		default:
		}
	}
}

func (ds *DiscoveryService) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ds.cleanup()
		}
	}
}

func (ds *DiscoveryService) cleanup() {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	now := time.Now()
	updated := false

	for name, game := range ds.games {
		if now.Sub(game.LastSeen) > 3*time.Second {
			delete(ds.games, name)
			updated = true
			log.Printf("Game disappeared: %s", name)
		}
	}

	if updated {
		select {
		case ds.eventCh <- DiscoveryEvent{Type: DiscoveryGamesUpdated}:
		default:
		}
	}
}
