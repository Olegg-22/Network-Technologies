package node

import (
	"context"
	"log"
	"net"
	"time"

	"lab4/internal/domain"
	"lab4/internal/network"
	pb "lab4/pkg/pb"
)

type ViewerConfig struct {
	Socket     *network.Socket
	MasterAddr *net.UDPAddr
	Config     *domain.GameConfig
	MyID       int32
	EventCh    chan<- Event
}

type Viewer struct {
	*BaseClient
}

func NewViewer(cfg ViewerConfig) *Viewer {
	base := NewBaseClient(BaseClientConfig{
		Socket:     cfg.Socket,
		MasterAddr: cfg.MasterAddr,
		Config:     cfg.Config,
		MyID:       cfg.MyID,
		EventCh:    cfg.EventCh,
	})

	return &Viewer{
		BaseClient: base,
	}
}

func (v *Viewer) Start(ctx context.Context) error {
	ctx, v.Cancel = context.WithCancel(ctx)

	v.UpdateLastReceived()
	v.UpdateLastSent()

	go v.ReceiveLoop(ctx, v.HandleMessage)
	go v.maintenanceLoop(ctx)

	go v.InitialPingBurst(ctx, 5)

	log.Printf("VIEWER started, watching MASTER at %s", v.MasterAddr)

	return nil
}

func (v *Viewer) Stop() error {
	if v.Cancel != nil {
		v.Cancel()
	}
	return nil
}

func (v *Viewer) GetRole() domain.NodeRole {
	return domain.RoleViewer
}

func (v *Viewer) GetState() *domain.GameState {
	return v.GetStateCopy()
}

func (v *Viewer) SendSteer(dir domain.Direction) error {
	return nil
}

func (v *Viewer) HandleMessage(msg *pb.GameMessage, from *net.UDPAddr) {
	v.UpdateLastReceived()

	switch msg.Type.(type) {
	case *pb.GameMessage_State:
		v.handleState(msg, from)
	case *pb.GameMessage_RoleChange:
		v.handleRoleChange(msg, from)
	case *pb.GameMessage_Ack:
		v.HandleAck(msg)
	case *pb.GameMessage_Ping:
		v.HandlePing(msg, from)
	case *pb.GameMessage_Error:
		v.handleError(msg, from)
	}
}

func (v *Viewer) handleError(msg *pb.GameMessage, from *net.UDPAddr) {
	errMsg := msg.GetError()
	log.Printf("VIEWER: Received error from %s: %s", from, errMsg.GetErrorMessage())

	v.SendAck(msg.GetMsgSeq(), from)

	if v.Cancel != nil {
		v.Cancel()
	}

	v.EventCh <- Event{Type: EventGameOver}
}

func (v *Viewer) maintenanceLoop(ctx context.Context) {
	interval := time.Duration(v.GameConfig.StateDelayMs/10) * time.Millisecond
	timeout := time.Duration(v.GameConfig.StateDelayMs*3) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lastRecv := v.GetLastReceived()
			lastSent := v.GetLastSent()

			if time.Since(lastRecv) > timeout {
				if v.DeputyAddr != nil {
					log.Println("VIEWER: MASTER timeout, switching to DEPUTY")
					v.MasterAddr = v.DeputyAddr
					v.DeputyAddr = nil
					v.UpdateLastReceived()
					v.SendPing()
				} else {
					log.Println("VIEWER: MASTER timeout, no DEPUTY available - game over")
					v.EventCh <- Event{Type: EventGameOver}
					return
				}
			}

			if time.Since(lastSent) > interval {
				v.SendPing()
			}
		}
	}
}

func (v *Viewer) handleState(msg *pb.GameMessage, from *net.UDPAddr) {
	state, updated := v.ParseStateMsg(msg)
	v.SendAck(msg.GetMsgSeq(), from)

	if !updated {
		return
	}

	v.UpdateDeputyFromState(state)

	v.EventCh <- Event{Type: EventStateUpdated}
}

func (v *Viewer) handleRoleChange(msg *pb.GameMessage, from *net.UDPAddr) {
	rc := msg.GetRoleChange()
	v.SendAck(msg.GetMsgSeq(), from)

	if rc.GetSenderRole() == pb.NodeRole_MASTER {
		log.Printf("VIEWER: New MASTER at %s", from)
		v.HandleNewMaster(from)
		v.SendPing()
		return
	}

	if rc.GetReceiverRole() == pb.NodeRole_VIEWER {
		log.Println("VIEWER: Confirmed as VIEWER")
	}
}
