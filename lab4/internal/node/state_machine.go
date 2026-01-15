package node

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"lab4/internal/domain"
	"lab4/internal/network"
	pb "lab4/pkg/pb"
)

type StateMachine struct {
	current Role
	mu      sync.RWMutex

	socket    *network.Socket
	multicast *network.Multicast
	eventCh   chan Event

	externalEventCh chan<- Event

	myID       int32
	playerName string
	gameName   string
	masterAddr *net.UDPAddr
	gameConfig *domain.GameConfig
	state      *domain.GameState

	ctx    context.Context
	cancel context.CancelFunc
}

func NewStateMachine(socket *network.Socket, multicast *network.Multicast, externalEventCh chan<- Event) *StateMachine {
	return &StateMachine{
		socket:          socket,
		multicast:       multicast,
		eventCh:         make(chan Event, 100),
		externalEventCh: externalEventCh,
	}
}

func (sm *StateMachine) Start(ctx context.Context) {
	sm.ctx, sm.cancel = context.WithCancel(ctx)

	go sm.eventForwarder()
}

func (sm *StateMachine) eventForwarder() {
	for {
		select {
		case <-sm.ctx.Done():
			return
		case event := <-sm.eventCh:
			switch event.Type {
			case EventRoleChanged:
				sm.handleRoleChange(event.Payload)
			case EventGameOver:

				sm.mu.Lock()
				if sm.current != nil {
					sm.current.Stop()
					sm.current = nil
				}

				sm.mu.Unlock()
				log.Println("StateMachine: Game over, stopped current role")
			}

			select {
			case sm.externalEventCh <- event:
			default:
				log.Println("External event channel full")
			}
		}
	}
}

func (sm *StateMachine) Stop() {
	if sm.cancel != nil {
		sm.cancel()
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.current != nil {
		sm.current.Stop()
		sm.current = nil
	}
}

func (sm *StateMachine) GetCurrentRole() Role {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

func (sm *StateMachine) GetState() *domain.GameState {
	sm.mu.RLock()
	role := sm.current
	sm.mu.RUnlock()

	if role == nil {
		return nil
	}
	return role.GetState()
}

func (sm *StateMachine) CreateGame(playerName, gameName string, config *domain.GameConfig) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.current != nil {
		sm.current.Stop()
	}

	state := domain.NewGameState(config)
	myID := int32(1)

	player := domain.NewPlayer(myID, playerName, domain.RoleMaster)
	state.AddPlayer(player)

	if !state.SpawnSnake(myID) {
		return fmt.Errorf("failed to spawn snake")
	}

	state.EnsureFood()

	sm.myID = myID
	sm.playerName = playerName
	sm.gameName = gameName
	sm.gameConfig = config
	sm.state = state

	log.Printf("Creating game: player=%s, game=%s, snakes=%d", playerName, gameName, len(state.Snakes))

	master := NewMaster(MasterConfig{
		Socket:     sm.socket,
		Multicast:  sm.multicast,
		State:      state,
		MyID:       myID,
		PlayerName: playerName,
		GameName:   gameName,
		EventCh:    sm.eventCh,
	})

	sm.current = master

	return master.Start(sm.ctx)
}

func (sm *StateMachine) JoinGame(masterAddr *net.UDPAddr, playerName, gameName string, config *domain.GameConfig, asViewer bool) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.current != nil {
		sm.current.Stop()
		sm.current = nil
	}

	sm.playerName = playerName
	sm.gameName = gameName
	sm.gameConfig = config
	sm.masterAddr = masterAddr

	requestedRole := domain.RoleNormal
	if asViewer {
		requestedRole = domain.RoleViewer
	}

	seq := sm.socket.NextSeq()
	joinMsg := network.BuildJoinMsg(seq, playerName, gameName, requestedRole)

	log.Printf("Sending JoinMsg to %s (seq=%d, name=%s)", masterAddr, seq, playerName)

	if err := sm.socket.Send(joinMsg, masterAddr); err != nil {
		return fmt.Errorf("failed to send join: %w", err)
	}

	for attempts := 0; attempts < 10; attempts++ {
		msg, addr, err := sm.socket.ReceiveWithTimeout(500 * time.Millisecond)
		if err != nil {
			log.Printf("Join attempt %d: timeout, retrying...", attempts+1)
			sm.socket.Send(joinMsg, masterAddr)
			continue
		}

		log.Printf("Received message from %s: type=%T", addr, msg.Type)

		switch msg.Type.(type) {
		case *pb.GameMessage_Ack:
			ack := msg
			log.Printf("Received ACK: msg_seq=%d, expected=%d, receiver_id=%d",
				ack.GetMsgSeq(), seq, ack.GetReceiverId())

			if ack.GetMsgSeq() == seq {
				myID := ack.GetReceiverId()
				sm.myID = myID
				sm.masterAddr = addr

				log.Printf("Joined game successfully, my ID: %d", myID)

				if asViewer {
					viewer := NewViewer(ViewerConfig{
						Socket:     sm.socket,
						MasterAddr: masterAddr,
						Config:     config,
						MyID:       myID,
						EventCh:    sm.eventCh,
					})
					sm.current = viewer
					return viewer.Start(sm.ctx)
				} else {
					normal := NewNormal(NormalConfig{
						Socket:     sm.socket,
						MasterAddr: masterAddr,
						Config:     config,
						MyID:       myID,
						EventCh:    sm.eventCh,
					})
					sm.current = normal
					return normal.Start(sm.ctx)
				}
			}

		case *pb.GameMessage_Error:
			errMsg := msg.GetError()
			return fmt.Errorf("join failed: %s", errMsg.GetErrorMessage())

		case *pb.GameMessage_Announcement:
			continue

		case *pb.GameMessage_State:
			continue

		case *pb.GameMessage_RoleChange:
			continue
		}
	}

	return fmt.Errorf("join timeout")
}

func (sm *StateMachine) TransitionToMaster(state *domain.GameState, peers *network.PeerManager) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	log.Println("StateMachine: TransitionToMaster")

	log.Println("StateMachine: Players in state before transition:")
	for _, player := range state.GetPlayersSlice() {
		log.Printf("  Player %d (%s): role=%v", player.ID, player.Name, player.Role)
	}

	if sm.current != nil {
		sm.current.Stop()
	}

	if player := state.GetPlayer(sm.myID); player != nil {
		player.Role = domain.RoleMaster
	}

	master := NewMaster(MasterConfig{
		Socket:     sm.socket,
		Multicast:  sm.multicast,
		State:      state,
		MyID:       sm.myID,
		PlayerName: sm.playerName,
		GameName:   sm.gameName,
		EventCh:    sm.eventCh,
	})

	if peers != nil {
		for _, peer := range peers.GetAll() {
			player := state.GetPlayer(peer.ID)
			if player == nil {
				log.Printf("StateMachine: Skipping peer %d - not in state", peer.ID)
				continue
			}
			if player.Role == domain.RoleViewer {
				log.Printf("StateMachine: Skipping peer %d - is VIEWER", peer.ID)
				continue
			}
			if player.Role == domain.RoleMaster && peer.ID != sm.myID {
				log.Printf("StateMachine: Skipping peer %d - is old MASTER", peer.ID)
				continue
			}

			peer.UpdateLastReceived()
			peer.UpdateLastSent()

			master.peers.Add(peer)

			log.Printf("StateMachine: Added peer %d (%s) to new MASTER", peer.ID, peer.Addr)
		}
	}

	sm.current = master
	sm.state = state

	return master.Start(sm.ctx)
}

func (sm *StateMachine) TransitionToViewer() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	log.Println("StateMachine: TransitionToViewer")

	var masterAddr *net.UDPAddr
	var currentState *domain.GameState

	if sm.current != nil {
		currentState = sm.current.GetState()

		switch role := sm.current.(type) {
		case *Master:
			if role.deputy != nil {
				masterAddr = role.deputy.Addr
				log.Printf("StateMachine: new MASTER will be at %s", masterAddr)
			}
		case *Deputy:
			masterAddr = role.MasterAddr
		case *Normal:
			masterAddr = role.MasterAddr
		}
	}

	if sm.current != nil {
		sm.current.Stop()
	}

	if masterAddr == nil {
		log.Println("StateMachine: no master address for VIEWER, going to menu")

		sm.current = nil
		sm.eventCh <- Event{Type: EventGameOver}
		return nil
	}

	viewer := NewViewer(ViewerConfig{
		Socket:     sm.socket,
		MasterAddr: masterAddr,
		Config:     sm.gameConfig,
		MyID:       sm.myID,
		EventCh:    sm.eventCh,
	})

	if currentState != nil {
		viewer.State = currentState
	}

	sm.current = viewer
	log.Printf("StateMachine: VIEWER started, watching MASTER at %s", masterAddr)

	return viewer.Start(sm.ctx)
}

func (sm *StateMachine) TransitionToDeputy(state *domain.GameState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	log.Printf("StateMachine: TransitionToDeputy, masterAddr=%s", sm.masterAddr)

	if sm.current != nil {
		sm.current.Stop()
	}

	deputy := NewDeputy(DeputyConfig{
		Socket:     sm.socket,
		Multicast:  sm.multicast,
		MasterAddr: sm.masterAddr,
		Config:     sm.gameConfig,
		State:      state,
		MyID:       sm.myID,
		GameName:   sm.gameName,
		EventCh:    sm.eventCh,
	})

	sm.current = deputy
	sm.state = state

	return deputy.Start(sm.ctx)
}

func (sm *StateMachine) TransitionToNormal(state *domain.GameState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	log.Printf("StateMachine: TransitionToNormal, masterAddr=%s", sm.masterAddr)

	if sm.current != nil {
		sm.current.Stop()
	}

	normal := NewNormal(NormalConfig{
		Socket:     sm.socket,
		MasterAddr: sm.masterAddr,
		Config:     sm.gameConfig,
		MyID:       sm.myID,
		EventCh:    sm.eventCh,
	})

	if state != nil {
		normal.State = state
	}

	sm.current = normal

	return normal.Start(sm.ctx)
}

func (sm *StateMachine) SendSteer(dir domain.Direction) error {
	sm.mu.RLock()
	role := sm.current
	sm.mu.RUnlock()

	if role == nil {
		return fmt.Errorf("no active role")
	}

	return role.SendSteer(dir)
}

func (sm *StateMachine) ExitGame() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.current != nil {
		if sm.current.GetRole() == domain.RoleNormal {
			if normal, ok := sm.current.(*Normal); ok {
				normal.ExitGame()
			}
		}
		sm.current.Stop()
		sm.current = nil
	}
}

func (sm *StateMachine) GetMyID() int32 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.myID
}

func (sm *StateMachine) handleRoleChange(payload interface{}) {
	switch data := payload.(type) {
	case RoleTransitionData:
		switch data.NewRole {
		case domain.RoleNormal:
			log.Printf("StateMachine: handling transition to NORMAL, new master at %s", data.MasterAddr)

			if data.MasterAddr != nil {
				sm.masterAddr = data.MasterAddr
			}

			err := sm.TransitionToNormal(data.State)
			if err != nil {
				log.Printf("StateMachine: failed to transition to NORMAL: %v", err)
			}

		case domain.RoleDeputy:
			log.Printf("StateMachine: handling transition to DEPUTY")

			if data.State != nil {
				log.Println("StateMachine: transitioning to DEPUTY with state")

				if data.MasterAddr != nil {
					sm.masterAddr = data.MasterAddr
				}

				err := sm.TransitionToDeputy(data.State)
				if err != nil {
					log.Printf("StateMachine: failed to transition to DEPUTY: %v", err)
				}
			}
		}

	case domain.NodeRole:
		log.Printf("StateMachine: handling role change to %v", data)

		switch data {
		case domain.RoleMaster:
			log.Println("StateMachine: transitioning to MASTER")

			sm.mu.RLock()
			currentRole := sm.current
			sm.mu.RUnlock()

			if currentRole == nil {
				log.Println("StateMachine: no current role for MASTER transition")
				return
			}

			state := currentRole.GetState()
			if state == nil {
				log.Println("StateMachine: no state for MASTER transition")
				return
			}

			var peers *network.PeerManager
			if deputy, ok := currentRole.(*Deputy); ok {
				peers = deputy.peers
			}

			err := sm.TransitionToMaster(state, peers)
			if err != nil {
				log.Printf("StateMachine: failed to transition to MASTER: %v", err)
			}

		case domain.RoleViewer:
			log.Println("StateMachine: transitioning to VIEWER")
			err := sm.TransitionToViewer()
			if err != nil {
				log.Printf("StateMachine: failed to transition to VIEWER: %v", err)
			}

		case domain.RoleDeputy:
			log.Println("StateMachine: RoleDeputy without state - ignoring")
		}
	}
}
