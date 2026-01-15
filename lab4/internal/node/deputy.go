package node

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"lab4/internal/domain"
	"lab4/internal/network"
	pb "lab4/pkg/pb"
)

type DeputyConfig struct {
	Socket     *network.Socket
	Multicast  *network.Multicast
	MasterAddr *net.UDPAddr
	Config     *domain.GameConfig
	State      *domain.GameState
	MyID       int32
	GameName   string
	EventCh    chan<- Event
}

type Deputy struct {
	*BaseClient

	multicast *network.Multicast
	peers     *network.PeerManager
	gameName  string

	becomingMaster bool
	imDead         bool
}

func NewDeputy(cfg DeputyConfig) *Deputy {
	base := NewBaseClient(BaseClientConfig{
		Socket:     cfg.Socket,
		MasterAddr: cfg.MasterAddr,
		Config:     cfg.Config,
		MyID:       cfg.MyID,
		EventCh:    cfg.EventCh,
	})

	base.State = cfg.State

	return &Deputy{
		BaseClient: base,
		multicast:  cfg.Multicast,
		peers:      network.NewPeerManager(),
		gameName:   cfg.GameName,
	}
}

func (d *Deputy) Start(ctx context.Context) error {
	ctx, d.Cancel = context.WithCancel(ctx)

	d.UpdateLastReceived()
	d.UpdateLastSent()

	go d.ReceiveLoop(ctx, d.HandleMessage)
	go d.maintenanceLoop(ctx)
	go d.RetryLoop(ctx)

	go d.InitialPingBurst(ctx, 5)

	log.Printf("DEPUTY started, my ID=%d, master at %s", d.MyID, d.MasterAddr)

	return nil
}

func (d *Deputy) Stop() error {
	if d.Cancel != nil {
		d.Cancel()
	}
	return nil
}

func (d *Deputy) GetRole() domain.NodeRole {
	return domain.RoleDeputy
}

func (d *Deputy) GetState() *domain.GameState {
	return d.GetStateCopy()
}

func (d *Deputy) SendSteer(dir domain.Direction) error {
	return d.BaseClient.SendSteer(dir)
}

func (d *Deputy) HandleMessage(msg *pb.GameMessage, from *net.UDPAddr) {
	d.UpdateLastReceived()

	switch msg.Type.(type) {
	case *pb.GameMessage_State:
		d.handleState(msg, from)
	case *pb.GameMessage_RoleChange:
		d.handleRoleChange(msg, from)
	case *pb.GameMessage_Ack:
		d.HandleAck(msg)
	case *pb.GameMessage_Ping:
		d.HandlePing(msg, from)
	}
}

func (d *Deputy) maintenanceLoop(ctx context.Context) {

	interval := time.Duration(d.GameConfig.StateDelayMs/10) * time.Millisecond

	timeout := time.Duration(float64(d.GameConfig.StateDelayMs)*0.8) * time.Millisecond

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lastRecv := d.GetLastReceived()
			lastSent := d.GetLastSent()

			if time.Since(lastRecv) > timeout {
				if !d.becomingMaster {
					log.Printf("DEPUTY: MASTER timeout detected (no message for %v)", time.Since(lastRecv))
					d.becomeMaster()
				}
				return
			}

			if time.Since(lastSent) > interval {
				d.SendPing()
			}
		}
	}
}

func (d *Deputy) handleState(msg *pb.GameMessage, from *net.UDPAddr) {
	stateMsg := msg.GetState()
	newOrder := stateMsg.GetState().GetStateOrder()

	d.StateMu.Lock()

	if d.State != nil && newOrder <= d.State.StateOrder {
		d.StateMu.Unlock()
		d.SendAck(msg.GetMsgSeq(), from)
		return
	}

	d.State = network.PbToGameState(stateMsg.GetState(), d.GameConfig)

	for _, player := range d.State.GetPlayersSlice() {

		if player.ID != d.MyID && player.IPAddress != "" &&
			player.Role != domain.RoleMaster && player.Role != domain.RoleViewer {

			addr := &net.UDPAddr{
				IP:   net.ParseIP(player.IPAddress),
				Port: int(player.Port),
			}

			peer := d.peers.Get(player.ID)
			if peer == nil {
				peer = network.NewPeer(player.ID, player.Name, addr, player.Role)
				d.peers.Add(peer)
			} else {
				d.peers.UpdateAddr(player.ID, addr)
				d.peers.UpdateRole(player.ID, player.Role)
			}
		} else if player.Role == domain.RoleViewer {
			d.peers.Remove(player.ID)
		}
	}

	d.StateMu.Unlock()

	d.SendAck(msg.GetMsgSeq(), from)
	d.EventCh <- Event{Type: EventStateUpdated}
}

func (d *Deputy) handleRoleChange(msg *pb.GameMessage, from *net.UDPAddr) {
	rc := msg.GetRoleChange()

	log.Printf("DEPUTY: Received RoleChange from %s: sender_role=%v, receiver_role=%v",
		from, rc.GetSenderRole(), rc.GetReceiverRole())

	d.SendAck(msg.GetMsgSeq(), from)

	if rc.GetReceiverRole() == pb.NodeRole_MASTER {
		log.Printf("DEPUTY: Received command to become MASTER from %s", from)
		if d.imDead {
			log.Println("DEPUTY: But I am already dead, cannot become MASTER")
			return
		}
		if !d.becomingMaster {
			d.becomeMaster()
		} else {
			log.Println("DEPUTY: Already becoming MASTER, ignoring duplicate")
		}
		return
	}

	if rc.GetReceiverRole() == pb.NodeRole_VIEWER {
		log.Println("DEPUTY: I am now VIEWER (my snake died)")
		d.imDead = true
		d.EventCh <- Event{Type: EventRoleChanged, Payload: domain.RoleViewer}
		return
	}

	if rc.GetSenderRole() == pb.NodeRole_VIEWER {
		log.Printf("DEPUTY: Master is leaving (sender_role=VIEWER), I should become MASTER")
		if d.imDead {
			log.Println("DEPUTY: But I am already dead, staying as VIEWER")
			return
		}
		if !d.becomingMaster {
			d.becomeMaster()
		}
		return
	}

	if rc.GetSenderRole() == pb.NodeRole_MASTER {
		if from.String() != d.MasterAddr.String() {
			log.Printf("DEPUTY: Another node %s became MASTER, transitioning to NORMAL", from)

			state := d.GetStateCopy()

			if d.Cancel != nil {
				d.Cancel()
			}

			d.EventCh <- Event{
				Type: EventRoleChanged,
				Payload: NormalTransitionData{
					NewRole:    domain.RoleNormal,
					State:      state,
					MasterAddr: from,
				},
			}
			return
		}
		log.Printf("DEPUTY: Ignoring sender_role=MASTER from current master without receiver_role")
	}
}

func (d *Deputy) becomeMaster() {
	if d.becomingMaster {
		return
	}
	d.becomingMaster = true

	log.Println("DEPUTY: Becoming MASTER now")

	d.StateMu.Lock()
	state := d.State

	if state == nil {
		d.StateMu.Unlock()
		log.Println("DEPUTY: No state available, cannot become MASTER")
		d.EventCh <- Event{Type: EventGameOver}
		return
	}

	//log.Printf("DEPUTY: Current players in state:")
	//for id, player := range state.Players {
	//	log.Printf("  Player %d (%s): role=%v", id, player.Name, player.Role)
	//}

	var oldMasterID int32 = -1

	for id, player := range state.Players {
		if id != d.MyID && player.IPAddress != "" {
			playerAddr := fmt.Sprintf("%s:%d", player.IPAddress, player.Port)
			if playerAddr == d.MasterAddr.String() {
				oldMasterID = id
				log.Printf("DEPUTY: Found old master by address: ID=%d (%s)", id, player.Name)
				break
			}
		}
	}

	for id, player := range state.Players {
		if id == d.MyID {
			continue
		}
		if (oldMasterID > 0 && id == oldMasterID) ||
			(oldMasterID < 0 && player.Role == domain.RoleMaster) {

			player.Role = domain.RoleViewer
			log.Printf("DEPUTY: Changed old MASTER (ID=%d, %s) role to VIEWER", id, player.Name)

			if snake, ok := state.Snakes[id]; ok && snake != nil {
				snake.State = domain.SnakeStateZombie
				log.Printf("DEPUTY: Marked snake of old MASTER (ID=%d) as ZOMBIE", id)
			}
			break
		}
	}

	if player, ok := state.Players[d.MyID]; ok && player != nil {
		player.Role = domain.RoleMaster
		log.Printf("DEPUTY: Set my role (ID=%d) to MASTER", d.MyID)
	}

	d.StateMu.Unlock()

	d.StateMu.RLock()
	playersToNotify := make([]*domain.Player, 0)
	for _, player := range state.Players {
		if player.ID == d.MyID {
			continue
		}
		if player.Role == domain.RoleViewer {
			log.Printf("DEPUTY: Skipping VIEWER %d", player.ID)
			continue
		}
		if player.IPAddress == "" || player.Port == 0 {
			log.Printf("DEPUTY: Player %d has no address, skipping", player.ID)
			continue
		}

		playersToNotify = append(playersToNotify, player.Copy())
	}
	d.StateMu.RUnlock()

	for _, player := range playersToNotify {
		addr := &net.UDPAddr{
			IP:   net.ParseIP(player.IPAddress),
			Port: int(player.Port),
		}

		if player.Role == domain.RoleNormal || player.Role == domain.RoleDeputy {
			peer := d.peers.Get(player.ID)

			if peer == nil {
				peer = network.NewPeer(player.ID, player.Name, addr, player.Role)
				d.peers.Add(peer)
				log.Printf("DEPUTY: Added peer %d (%s) from state", player.ID, addr)
			} else {
				d.peers.UpdateAddr(player.ID, addr)
			}

			peer.UpdateLastReceived()
			peer.UpdateLastSent()
		}

		seq := d.Socket.NextSeq()
		masterRole := domain.RoleMaster
		msg := network.BuildRoleChangeMsg(seq, d.MyID, player.ID, &masterRole, nil)
		d.Socket.Send(msg, addr)
		d.AckTracker.Track(seq, msg, addr)
		log.Printf("DEPUTY: Sent RoleChange (I am MASTER) to player %d at %s", player.ID, addr)
	}

	d.EventCh <- Event{Type: EventRoleChanged, Payload: domain.RoleMaster}
}
