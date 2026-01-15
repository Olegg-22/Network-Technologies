package node

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"lab4/internal/domain"
	"lab4/internal/network"
	pb "lab4/pkg/pb"
)

type BaseClientConfig struct {
	Socket     *network.Socket
	MasterAddr *net.UDPAddr
	Config     *domain.GameConfig
	MyID       int32
	EventCh    chan<- Event
}

type BaseClient struct {
	Socket     *network.Socket
	AckTracker *network.AckTracker

	MyID       int32
	MasterAddr *net.UDPAddr
	DeputyAddr *net.UDPAddr

	State      *domain.GameState
	GameConfig *domain.GameConfig
	StateMu    sync.RWMutex

	LastReceived time.Time
	LastSent     time.Time
	TimeMu       sync.Mutex

	EventCh chan<- Event
	Cancel  context.CancelFunc
}

func NewBaseClient(cfg BaseClientConfig) *BaseClient {
	retryInterval := time.Duration(cfg.Config.StateDelayMs/10) * time.Millisecond

	return &BaseClient{
		Socket:       cfg.Socket,
		AckTracker:   network.NewAckTracker(retryInterval),
		MyID:         cfg.MyID,
		MasterAddr:   cfg.MasterAddr,
		GameConfig:   cfg.Config,
		EventCh:      cfg.EventCh,
		LastReceived: time.Now(),
		LastSent:     time.Now(),
	}
}

func (b *BaseClient) UpdateLastReceived() {
	b.TimeMu.Lock()
	b.LastReceived = time.Now()
	b.TimeMu.Unlock()
}

func (b *BaseClient) UpdateLastSent() {
	b.TimeMu.Lock()
	b.LastSent = time.Now()
	b.TimeMu.Unlock()
}

func (b *BaseClient) GetLastReceived() time.Time {
	b.TimeMu.Lock()
	defer b.TimeMu.Unlock()
	return b.LastReceived
}

func (b *BaseClient) GetLastSent() time.Time {
	b.TimeMu.Lock()
	defer b.TimeMu.Unlock()
	return b.LastSent
}

func (b *BaseClient) GetStateCopy() *domain.GameState {
	b.StateMu.RLock()
	defer b.StateMu.RUnlock()
	if b.State == nil {
		return nil
	}
	return b.State.Copy()
}

func (b *BaseClient) SetState(state *domain.GameState) {
	b.StateMu.Lock()
	b.State = state
	b.StateMu.Unlock()
}

func (b *BaseClient) SendAck(seq int64, addr *net.UDPAddr) {
	ackMsg := network.BuildAckMsg(seq, b.MyID, 0)
	b.Socket.Send(ackMsg, addr)
	b.UpdateLastSent()
}

func (b *BaseClient) SendPing() {
	seq := b.Socket.NextSeq()
	pingMsg := network.BuildPingMsg(seq, b.MyID, 0)
	b.Socket.Send(pingMsg, b.MasterAddr)
	b.AckTracker.Track(seq, pingMsg, b.MasterAddr)
	b.UpdateLastSent()
}

func (b *BaseClient) SendSteer(dir domain.Direction) error {
	seq := b.Socket.NextSeq()
	msg := network.BuildSteerMsg(seq, dir, b.MyID)

	if err := b.Socket.Send(msg, b.MasterAddr); err != nil {
		log.Printf("BaseClient: failed to send steer: %v", err)
		return err
	}

	b.AckTracker.Track(seq, msg, b.MasterAddr)
	b.UpdateLastSent()
	return nil
}

func (b *BaseClient) ReceiveLoop(ctx context.Context, handler func(msg *pb.GameMessage, addr *net.UDPAddr)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, addr, err := b.Socket.ReceiveWithTimeout(100 * time.Millisecond)
		if err != nil {
			continue
		}

		handler(msg, addr)
	}
}

func (b *BaseClient) RetryLoop(ctx context.Context) {
	interval := time.Duration(b.GameConfig.StateDelayMs/10) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			retries := b.AckTracker.GetRetries()
			for _, pm := range retries {
				b.Socket.Send(pm.Message, pm.Target)
			}
		}
	}
}

func (b *BaseClient) UpdateDeputyFromState(state *domain.GameState) {
	b.DeputyAddr = nil
	for _, player := range state.GetPlayersSlice() {
		if player.Role == domain.RoleDeputy && player.IPAddress != "" && player.ID != b.MyID {
			b.DeputyAddr = &net.UDPAddr{
				IP:   net.ParseIP(player.IPAddress),
				Port: int(player.Port),
			}
			break
		}
	}
}

func (b *BaseClient) HandlePing(msg *pb.GameMessage, from *net.UDPAddr) {
	b.SendAck(msg.GetMsgSeq(), from)
}

func (b *BaseClient) HandleAck(msg *pb.GameMessage) {
	b.AckTracker.Acknowledge(msg.GetMsgSeq())
}

func (b *BaseClient) HandleNewMaster(from *net.UDPAddr) {
	log.Printf("BaseClient: New MASTER at %s", from)
	b.MasterAddr = from
	b.DeputyAddr = nil
	b.AckTracker.RedirectTo(from)
	b.UpdateLastReceived()
}

func (b *BaseClient) SwitchToDeputyOnMasterTimeout() bool {
	if b.DeputyAddr != nil {
		log.Printf("BaseClient: Switching to DEPUTY at %s", b.DeputyAddr)
		b.MasterAddr = b.DeputyAddr
		b.DeputyAddr = nil
		b.AckTracker.RedirectTo(b.MasterAddr)
		return true
	}
	log.Println("BaseClient: No DEPUTY available")
	return false
}

func (b *BaseClient) ExitGame() {
	seq := b.Socket.NextSeq()
	viewerRole := domain.RoleViewer

	msg := network.BuildRoleChangeMsg(seq, b.MyID, 0, &viewerRole, nil)

	b.Socket.Send(msg, b.MasterAddr)
	b.AckTracker.Track(seq, msg, b.MasterAddr)
}

func (b *BaseClient) ParseStateMsg(msg *pb.GameMessage) (*domain.GameState, bool) {
	stateMsg := msg.GetState()
	newOrder := stateMsg.GetState().GetStateOrder()

	b.StateMu.Lock()
	defer b.StateMu.Unlock()

	if b.State != nil && newOrder <= b.State.StateOrder {
		return nil, false
	}

	b.State = network.PbToGameState(stateMsg.GetState(), b.GameConfig)
	return b.State.Copy(), true
}

func (b *BaseClient) InitialPingBurst(ctx context.Context, count int) {
	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			b.SendPing()
			time.Sleep(100 * time.Millisecond)
		}
	}
}
