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

type NormalConfig struct {
	Socket     *network.Socket
	MasterAddr *net.UDPAddr
	Config     *domain.GameConfig
	MyID       int32
	EventCh    chan<- Event
}

type Normal struct {
	*BaseClient

	shouldBecomeDeputy bool
	deputyMu           sync.Mutex

	transitioning bool
	transitionMu  sync.Mutex
}

func NewNormal(cfg NormalConfig) *Normal {
	base := NewBaseClient(BaseClientConfig{
		Socket:     cfg.Socket,
		MasterAddr: cfg.MasterAddr,
		Config:     cfg.Config,
		MyID:       cfg.MyID,
		EventCh:    cfg.EventCh,
	})

	return &Normal{
		BaseClient: base,
	}
}

func (n *Normal) Start(ctx context.Context) error {
	ctx, n.Cancel = context.WithCancel(ctx)

	go n.ReceiveLoop(ctx, n.HandleMessage)
	go n.maintenanceLoop(ctx)
	go n.RetryLoop(ctx)

	log.Printf("NORMAL started, my ID=%d, master at %s", n.MyID, n.MasterAddr)

	return nil
}

func (n *Normal) Stop() error {
	if n.Cancel != nil {
		n.Cancel()
	}
	return nil
}

func (n *Normal) GetRole() domain.NodeRole {
	return domain.RoleNormal
}

func (n *Normal) GetState() *domain.GameState {
	return n.GetStateCopy()
}

func (n *Normal) SendSteer(dir domain.Direction) error {
	return n.BaseClient.SendSteer(dir)
}

func (n *Normal) HandleMessage(msg *pb.GameMessage, from *net.UDPAddr) {
	n.UpdateLastReceived()

	switch msg.Type.(type) {
	case *pb.GameMessage_State:
		n.handleState(msg, from)
	case *pb.GameMessage_RoleChange:
		n.handleRoleChange(msg, from)
	case *pb.GameMessage_Ack:
		n.HandleAck(msg)
	case *pb.GameMessage_Ping:
		n.HandlePing(msg, from)
	case *pb.GameMessage_Error:
		n.handleError(msg)
	}
}

func (n *Normal) maintenanceLoop(ctx context.Context) {

	interval := time.Duration(n.GameConfig.StateDelayMs/10) * time.Millisecond

	timeout := time.Duration(float64(n.GameConfig.StateDelayMs)*0.8) * time.Millisecond

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lastRecv := n.GetLastReceived()
			lastSent := n.GetLastSent()

			if time.Since(lastRecv) > timeout {
				n.handleMasterTimeout()
			}

			if time.Since(lastSent) > interval {
				n.SendPing()
			}
		}
	}
}

func (n *Normal) handleState(msg *pb.GameMessage, from *net.UDPAddr) {
	state, updated := n.ParseStateMsg(msg)
	n.SendAck(msg.GetMsgSeq(), from)

	if !updated {
		return
	}

	n.UpdateDeputyFromState(state)

	n.deputyMu.Lock()
	shouldTransition := n.shouldBecomeDeputy
	n.deputyMu.Unlock()

	if shouldTransition {
		if n.tryTransition() {
			log.Printf("NORMAL: Now have state, transitioning to DEPUTY")
			n.Cancel()
			n.EventCh <- Event{
				Type: EventRoleChanged,
				Payload: DeputyTransitionData{
					NewRole:    domain.RoleDeputy,
					State:      state,
					MasterAddr: n.MasterAddr,
				},
			}
		}
		return
	}

	for _, player := range state.GetPlayersSlice() {
		if player.ID == n.MyID && player.Role == domain.RoleDeputy {
			if n.tryTransition() {
				log.Printf("NORMAL: State shows I am DEPUTY, transitioning now")
				n.Cancel()
				n.EventCh <- Event{
					Type: EventRoleChanged,
					Payload: DeputyTransitionData{
						NewRole:    domain.RoleDeputy,
						State:      state,
						MasterAddr: n.MasterAddr,
					},
				}
			}
			return
		}
	}

	n.EventCh <- Event{Type: EventStateUpdated}
}

func (n *Normal) handleRoleChange(msg *pb.GameMessage, from *net.UDPAddr) {
	rc := msg.GetRoleChange()
	n.SendAck(msg.GetMsgSeq(), from)

	if rc.GetSenderRole() == pb.NodeRole_MASTER {
		n.HandleNewMaster(from)
	}

	if rc.GetReceiverRole() == pb.NodeRole_DEPUTY {
		if !n.tryTransition() {
			log.Println("NORMAL: Already transitioning, ignoring duplicate RoleChange")
			return
		}

		log.Println("NORMAL: RoleChange says I am now DEPUTY")

		state := n.GetStateCopy()
		if state != nil {
			log.Printf("NORMAL: Have state, transitioning to DEPUTY immediately, masterAddr=%s", n.MasterAddr)
			if n.Cancel != nil {
				n.Cancel()
			}
			n.EventCh <- Event{
				Type: EventRoleChanged,
				Payload: DeputyTransitionData{
					NewRole:    domain.RoleDeputy,
					State:      state,
					MasterAddr: n.MasterAddr,
				},
			}
		} else {
			log.Println("NORMAL: No state yet, will transition when state arrives")
			n.deputyMu.Lock()
			n.shouldBecomeDeputy = true
			n.deputyMu.Unlock()

			n.transitionMu.Lock()
			n.transitioning = false
			n.transitionMu.Unlock()
		}
		return
	}

	if rc.GetSenderRole() == pb.NodeRole_MASTER {
		n.SendPing()
		log.Printf("NORMAL: Sent ping to new MASTER at %s", from)
		return
	}

	if rc.GetReceiverRole() == pb.NodeRole_VIEWER {
		log.Println("NORMAL: I am now VIEWER (my snake died)")
		n.EventCh <- Event{Type: EventRoleChanged, Payload: domain.RoleViewer}
	}
}

func (n *Normal) handleError(msg *pb.GameMessage) {
	errMsg := msg.GetError()
	n.EventCh <- Event{
		Type:    EventError,
		Payload: ErrorPayload{Message: errMsg.GetErrorMessage()},
	}
}

func (n *Normal) handleMasterTimeout() {
	log.Println("NORMAL: MASTER timeout")

	if n.SwitchToDeputyOnMasterTimeout() {
		return
	}

	log.Println("NORMAL: No DEPUTY available, game over")
	n.EventCh <- Event{Type: EventGameOver}
	if n.Cancel != nil {
		n.Cancel()
	}
}

func (n *Normal) tryTransition() bool {
	n.transitionMu.Lock()
	defer n.transitionMu.Unlock()

	if n.transitioning {
		return false
	}
	n.transitioning = true
	return true
}

func (n *Normal) ExitGame() {
	n.BaseClient.ExitGame()
}
