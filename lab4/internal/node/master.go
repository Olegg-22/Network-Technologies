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

type MasterConfig struct {
	Socket     *network.Socket
	Multicast  *network.Multicast
	State      *domain.GameState
	MyID       int32
	PlayerName string
	GameName   string
	EventCh    chan<- Event
}

type Master struct {
	config MasterConfig

	socket     *network.Socket
	multicast  *network.Multicast
	peers      *network.PeerManager
	ackTracker *network.AckTracker

	state    *domain.GameState
	myID     int32
	gameName string

	pendingMoves map[int32]pendingMove
	movesMu      sync.Mutex

	eventCh chan<- Event
	cancel  context.CancelFunc

	deputy *network.Peer

	mu sync.RWMutex

	startTime time.Time

	isTransferringToDep bool
}

type pendingMove struct {
	direction domain.Direction
	msgSeq    int64
}

func NewMaster(cfg MasterConfig) *Master {
	retryInterval := time.Duration(cfg.State.Config.StateDelayMs/10) * time.Millisecond

	return &Master{
		config:       cfg,
		socket:       cfg.Socket,
		multicast:    cfg.Multicast,
		peers:        network.NewPeerManager(),
		ackTracker:   network.NewAckTracker(retryInterval),
		state:        cfg.State,
		myID:         cfg.MyID,
		gameName:     cfg.GameName,
		pendingMoves: make(map[int32]pendingMove),
		eventCh:      cfg.EventCh,
		startTime:    time.Now(),
	}
}

func (m *Master) Start(ctx context.Context) error {
	ctx, m.cancel = context.WithCancel(ctx)
	m.startTime = time.Now()

	if m.deputy == nil {
		m.selectNewDeputy()
	}

	go m.receiveLoop(ctx)
	go m.tickLoop(ctx)
	go m.announceLoop(ctx)
	go m.maintenanceLoop(ctx)
	go m.retryLoop(ctx)

	m.eventCh <- Event{Type: EventStateUpdated}

	log.Printf("MASTER started, listening on port %d", m.socket.LocalPort())

	return nil
}

func (m *Master) Stop() error {
	if m.deputy != nil && !m.isTransferringToDep {
		log.Printf("MASTER: Transferring control to DEPUTY %d (graceful exit)", m.deputy.ID)

		seq := m.socket.NextSeq()
		viewerRole := domain.RoleViewer
		masterRole := domain.RoleMaster
		roleMsg := network.BuildRoleChangeMsg(seq, m.myID, m.deputy.ID, &viewerRole, &masterRole)
		m.socket.Send(roleMsg, m.deputy.Addr)

		time.Sleep(100 * time.Millisecond)
	}

	if m.cancel != nil {
		m.cancel()
	}
	return nil
}

func (m *Master) GetRole() domain.NodeRole {
	return domain.RoleMaster
}

func (m *Master) GetState() *domain.GameState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.Copy()
}

func (m *Master) SendSteer(dir domain.Direction) error {
	m.movesMu.Lock()
	defer m.movesMu.Unlock()

	m.pendingMoves[m.myID] = pendingMove{
		direction: dir,
		msgSeq:    time.Now().UnixNano(),
	}
	return nil
}

func (m *Master) HandleMessage(msg *pb.GameMessage, from *net.UDPAddr) {
	peer := m.peers.GetByAddr(from)
	if peer != nil {
		peer.UpdateLastReceived()
	}

	switch msg.Type.(type) {
	case *pb.GameMessage_Steer:
		m.handleSteer(msg, from)
	case *pb.GameMessage_Join:
		log.Printf("MASTER: Received JoinMsg from %s", from)
		m.handleJoin(msg, from)
	case *pb.GameMessage_Ping:
		m.handlePing(msg, from)
	case *pb.GameMessage_Ack:
		m.ackTracker.Acknowledge(msg.GetMsgSeq())
	case *pb.GameMessage_RoleChange:
		m.handleRoleChange(msg, from)
	case *pb.GameMessage_Discover:
		m.handleDiscover(msg, from)
	}
}

func (m *Master) receiveLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, addr, err := m.socket.ReceiveWithTimeout(100 * time.Millisecond)
		if err != nil {
			continue
		}

		m.HandleMessage(msg, addr)
	}
}

func (m *Master) tickLoop(ctx context.Context) {
	interval := time.Duration(m.state.Config.StateDelayMs) * time.Millisecond

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.doTick()
		}
	}
}

func (m *Master) doTick() {
	m.movesMu.Lock()
	moves := make(map[int32]domain.Direction)
	for playerID, pm := range m.pendingMoves {
		moves[playerID] = pm.direction
	}
	m.pendingMoves = make(map[int32]pendingMove)
	m.movesMu.Unlock()

	m.mu.Lock()
	result := m.state.Tick(moves)
	m.mu.Unlock()

	masterDied := false
	for _, playerID := range result.KilledPlayers {
		if playerID == m.myID {
			masterDied = true
			break
		}
	}

	for _, playerID := range result.KilledPlayers {
		if playerID == m.myID {
			continue
		}

		if m.deputy != nil && m.deputy.ID == playerID {
			log.Printf("MASTER: DEPUTY %d died, electing new one", playerID)

			m.selectNewDeputy()
		}

		if peer := m.peers.Get(playerID); peer != nil {
			m.sendRoleChange(peer, domain.RoleViewer)
		}

		m.eventCh <- Event{Type: EventPlayerLeft, Payload: playerID}
	}

	if masterDied {
		log.Println("MASTER: I died! Transferring to DEPUTY and becoming VIEWER")
		m.handleMasterDeath()
		return
	}

	m.broadcastState()
	m.eventCh <- Event{Type: EventStateUpdated}
}

func (m *Master) handleMasterDeath() {
	if m.deputy == nil {
		log.Println("MASTER: No DEPUTY available, notifying all peers and ending game")

		for _, peer := range m.peers.GetAll() {
			seq := m.socket.NextSeq()
			errMsg := network.BuildErrorMsg(seq, "Game Over - no players left")

			m.socket.Send(errMsg, peer.Addr)

			log.Printf("MASTER: Sent GameOver to peer %d at %s", peer.ID, peer.Addr)
		}

		if m.cancel != nil {
			m.cancel()
		}

		m.eventCh <- Event{Type: EventGameOver}
		return
	}

	m.isTransferringToDep = true

	m.broadcastState()
	log.Println("MASTER: Sent final state before transferring control")

	seq := m.socket.NextSeq()
	viewerRole := domain.RoleViewer
	masterRole := domain.RoleMaster
	roleMsg := network.BuildRoleChangeMsg(seq, m.myID, m.deputy.ID, &viewerRole, &masterRole)

	m.socket.Send(roleMsg, m.deputy.Addr)
	m.ackTracker.Track(seq, roleMsg, m.deputy.Addr)

	log.Printf("MASTER: Transferred control to DEPUTY %d, becoming VIEWER", m.deputy.ID)

	m.eventCh <- Event{Type: EventRoleChanged, Payload: domain.RoleViewer}
}

func (m *Master) sendRoleChange(peer *network.Peer, newRole domain.NodeRole) {
	seq := m.socket.NextSeq()
	roleMsg := network.BuildRoleChangeMsg(seq, m.myID, peer.ID, nil, &newRole)

	m.socket.Send(roleMsg, peer.Addr)
	m.ackTracker.Track(seq, roleMsg, peer.Addr)

	log.Printf("MASTER: Sent RoleChange to peer %d: now %v", peer.ID, newRole)
}

func (m *Master) broadcastState() {
	m.mu.RLock()
	state := m.state.Copy()
	m.mu.RUnlock()

	for _, peer := range m.peers.GetAll() {
		seq := m.socket.NextSeq()
		msg := network.BuildStateMsg(seq, state)

		if err := m.socket.Send(msg, peer.Addr); err != nil {
			log.Printf("Failed to send state to %s: %v", peer.Addr, err)
			continue
		}

		m.ackTracker.Track(seq, msg, peer.Addr)
		peer.UpdateLastSent()
	}
}

func (m *Master) announceLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.sendAnnouncement()
		}
	}
}

func (m *Master) sendAnnouncement() {
	m.mu.RLock()
	state := m.state.Copy()
	m.mu.RUnlock()

	_, _, canJoin := m.state.FindSpawnPlace()

	seq := m.socket.NextSeq()
	msg := network.BuildAnnouncementMsg(seq, m.gameName, state, canJoin)

	if err := m.socket.SendMulticast(msg); err != nil {
		log.Printf("Failed to send announcement: %v", err)
	}
}

func (m *Master) maintenanceLoop(ctx context.Context) {
	interval := time.Duration(m.state.Config.StateDelayMs/10) * time.Millisecond
	timeout := time.Duration(float64(m.state.Config.StateDelayMs)*0.8) * time.Millisecond
	gracePeriod := time.Duration(m.state.Config.StateDelayMs*3) * time.Millisecond

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Since(m.startTime) < gracePeriod {

				needPing := m.peers.FindNeedPing(interval)

				for _, peer := range needPing {
					m.sendPing(peer)
				}
				continue
			}

			timedOut := m.peers.FindTimedOut(timeout)
			for _, peer := range timedOut {
				m.handlePeerTimeout(peer)
			}

			needPing := m.peers.FindNeedPing(interval)
			for _, peer := range needPing {
				m.sendPing(peer)
			}
		}
	}
}

func (m *Master) retryLoop(ctx context.Context) {
	interval := time.Duration(m.state.Config.StateDelayMs/10) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:

			retries := m.ackTracker.GetRetries()

			for _, pm := range retries {
				m.socket.Send(pm.Message, pm.Target)
			}
		}
	}
}

func (m *Master) handleSteer(msg *pb.GameMessage, from *net.UDPAddr) {
	peer := m.peers.GetByAddr(from)
	if peer == nil {
		log.Printf("MASTER: Steer from unknown peer %s", from)
		return
	}

	steer := msg.GetSteer()
	newDir := network.PbToDirection(steer.GetDirection())

	m.mu.RLock()
	snake := m.state.GetSnake(peer.ID)
	m.mu.RUnlock()

	if snake != nil && newDir.IsOpposite(snake.HeadDirection) {
		m.sendAckTo(msg.GetMsgSeq(), peer.ID, from)
		return
	}

	m.movesMu.Lock()
	if existing, ok := m.pendingMoves[peer.ID]; ok {
		if msg.GetMsgSeq() <= existing.msgSeq {
			m.movesMu.Unlock()
			m.sendAckTo(msg.GetMsgSeq(), peer.ID, from)
			return
		}
	}

	m.pendingMoves[peer.ID] = pendingMove{
		direction: newDir,
		msgSeq:    msg.GetMsgSeq(),
	}
	m.movesMu.Unlock()

	m.sendAckTo(msg.GetMsgSeq(), peer.ID, from)
}

func (m *Master) handleJoin(msg *pb.GameMessage, from *net.UDPAddr) {
	join := msg.GetJoin()
	playerName := join.GetPlayerName()
	requestedRole := domain.NodeRole(join.GetRequestedRole())

	log.Printf("MASTER: Processing join from %s (name=%s, role=%v)", from, playerName, requestedRole)

	newID := m.state.GeneratePlayerID()

	player := domain.NewPlayer(newID, playerName, requestedRole)
	player.IPAddress = from.IP.String()
	player.Port = int32(from.Port)

	if requestedRole != domain.RoleViewer {
		if !m.state.SpawnSnake(newID) {
			log.Printf("MASTER: No space for snake, sending error to %s", from)
			seq := m.socket.NextSeq()
			errMsg := network.BuildErrorMsg(seq, "No space to spawn snake")

			m.socket.Send(errMsg, from)
			return
		}
		player.Role = domain.RoleNormal
	}

	m.state.AddPlayer(player)

	peer := network.NewPeer(newID, playerName, from, player.Role)
	m.peers.Add(peer)

	if m.deputy == nil && player.Role == domain.RoleNormal {
		m.deputy = peer
		player.Role = domain.RoleDeputy
		peer.Role = domain.RoleDeputy
		m.peers.UpdateRole(newID, domain.RoleDeputy)

		if p := m.state.GetPlayer(newID); p != nil {
			p.Role = domain.RoleDeputy
		}

		log.Printf("MASTER: %s is now DEPUTY", playerName)

		seq := m.socket.NextSeq()
		deputyRole := domain.RoleDeputy
		roleMsg := network.BuildRoleChangeMsg(seq, m.myID, newID, nil, &deputyRole)

		m.socket.Send(roleMsg, from)
		m.ackTracker.Track(seq, roleMsg, from)
	}

	ackMsg := network.BuildAckMsg(msg.GetMsgSeq(), m.myID, newID)
	if err := m.socket.Send(ackMsg, from); err != nil {
		log.Printf("MASTER: Failed to send ACK: %v", err)
	} else {
		log.Printf("MASTER: Sent ACK to %s (playerID=%d)", from, newID)
	}

	log.Printf("MASTER: Player %s (ID: %d) joined", playerName, newID)

	m.eventCh <- Event{Type: EventPlayerJoined, Payload: player}
}

func (m *Master) handlePing(msg *pb.GameMessage, from *net.UDPAddr) {
	peer := m.peers.GetByAddr(from)
	if peer != nil {
		peer.UpdateLastReceived()
		m.sendAckTo(msg.GetMsgSeq(), peer.ID, from)
		return
	}

	senderID := msg.GetSenderId()
	if senderID > 0 {
		peer = m.peers.Get(senderID)
		if peer != nil {
			log.Printf("MASTER: Updating peer %d address from %s to %s", senderID, peer.Addr, from)
			m.peers.UpdateAddr(senderID, from)
			peer.UpdateLastReceived()
			m.sendAckTo(msg.GetMsgSeq(), peer.ID, from)
			return
		}

		m.mu.RLock()
		player := m.state.GetPlayer(senderID)
		m.mu.RUnlock()

		if player != nil {
			m.mu.Lock()
			if statePlayer := m.state.GetPlayer(player.ID); statePlayer != nil {
				statePlayer.IPAddress = from.IP.String()
				statePlayer.Port = int32(from.Port)
			}
			m.mu.Unlock()

			peer = network.NewPeer(player.ID, player.Name, from, player.Role)
			m.peers.Add(peer)

			log.Printf("MASTER: Added peer %d (%s) role=%v from state via ping", player.ID, from, player.Role)

			seq := m.socket.NextSeq()
			masterRole := domain.RoleMaster
			roleMsg := network.BuildRoleChangeMsg(seq, m.myID, player.ID, &masterRole, nil)

			m.socket.Send(roleMsg, from)
			m.ackTracker.Track(seq, roleMsg, from)

			m.sendAckTo(msg.GetMsgSeq(), player.ID, from)
			return
		}

		log.Printf("MASTER: Adding unknown sender_id=%d as VIEWER from %s", senderID, from)
		peer = network.NewPeer(senderID, fmt.Sprintf("Viewer%d", senderID), from, domain.RoleViewer)
		m.peers.Add(peer)

		m.mu.Lock()
		viewerPlayer := domain.NewPlayer(senderID, fmt.Sprintf("Viewer%d", senderID), domain.RoleViewer)
		viewerPlayer.IPAddress = from.IP.String()
		viewerPlayer.Port = int32(from.Port)
		m.state.AddPlayer(viewerPlayer)
		m.mu.Unlock()

		seq := m.socket.NextSeq()
		masterRole := domain.RoleMaster
		roleMsg := network.BuildRoleChangeMsg(seq, m.myID, senderID, &masterRole, nil)

		m.socket.Send(roleMsg, from)
		m.ackTracker.Track(seq, roleMsg, from)

		m.sendAckTo(msg.GetMsgSeq(), senderID, from)
		return
	}

	log.Printf("MASTER: Ping from completely unknown peer %s (no sender_id)", from)
	ackMsg := network.BuildAckMsg(msg.GetMsgSeq(), m.myID, 0)
	m.socket.Send(ackMsg, from)
}

func (m *Master) handleRoleChange(msg *pb.GameMessage, from *net.UDPAddr) {
	peer := m.peers.GetByAddr(from)
	if peer == nil {
		return
	}

	rc := msg.GetRoleChange()
	if rc.GetSenderRole() == pb.NodeRole_VIEWER {
		m.handlePlayerExit(peer)
	}

	m.sendAckTo(msg.GetMsgSeq(), peer.ID, from)
}

func (m *Master) handleDiscover(msg *pb.GameMessage, from *net.UDPAddr) {
	m.mu.RLock()
	state := m.state.Copy()
	m.mu.RUnlock()

	_, _, canJoin := m.state.FindSpawnPlace()

	seq := m.socket.NextSeq()
	announcement := network.BuildAnnouncementMsg(seq, m.gameName, state, canJoin)
	m.socket.Send(announcement, from)
}

func (m *Master) handlePlayerExit(peer *network.Peer) {
	m.state.MakeSnakeZombie(peer.ID)

	if player := m.state.GetPlayer(peer.ID); player != nil {
		player.Role = domain.RoleViewer
	}

	if m.deputy != nil && m.deputy.ID == peer.ID {
		m.selectNewDeputy()
	}

	m.eventCh <- Event{Type: EventPlayerLeft, Payload: peer.ID}
}

func (m *Master) handlePeerTimeout(peer *network.Peer) {
	log.Printf("Peer %d timed out", peer.ID)

	m.ackTracker.RemoveByAddr(peer.Addr)
	m.state.MakeSnakeZombie(peer.ID)

	m.peers.Remove(peer.ID)

	if m.deputy != nil && m.deputy.ID == peer.ID {
		m.selectNewDeputy()
	}

	m.eventCh <- Event{Type: EventPlayerLeft, Payload: peer.ID}
}

func (m *Master) selectNewDeputy() {
	m.deputy = nil

	for _, peer := range m.peers.GetAll() {
		player := m.state.GetPlayer(peer.ID)
		if player == nil {
			continue
		}

		if player.Role == domain.RoleNormal {
			m.deputy = peer
			peer.Role = domain.RoleDeputy
			m.peers.UpdateRole(peer.ID, domain.RoleDeputy)

			player.Role = domain.RoleDeputy

			seq := m.socket.NextSeq()
			deputyRole := domain.RoleDeputy
			roleMsg := network.BuildRoleChangeMsg(seq, m.myID, peer.ID, nil, &deputyRole)
			m.socket.Send(roleMsg, peer.Addr)
			m.ackTracker.Track(seq, roleMsg, peer.Addr)

			log.Printf("MASTER: Elected new DEPUTY: %d (%s)", peer.ID, peer.Name)
			return
		}
	}

	log.Println("MASTER: No NORMAL peers available for DEPUTY role")
}

func (m *Master) sendAckTo(seq int64, receiverID int32, addr *net.UDPAddr) {
	ackMsg := network.BuildAckMsg(seq, m.myID, receiverID)
	m.socket.Send(ackMsg, addr)
}

func (m *Master) sendAck(seq int64, peer *network.Peer) {
	ackMsg := network.BuildAckMsg(seq, m.myID, peer.ID)
	m.socket.Send(ackMsg, peer.Addr)
	peer.UpdateLastSent()
}

func (m *Master) sendPing(peer *network.Peer) {
	seq := m.socket.NextSeq()
	pingMsg := network.BuildPingMsg(seq, m.myID, peer.ID)
	m.socket.Send(pingMsg, peer.Addr)
	m.ackTracker.Track(seq, pingMsg, peer.Addr)
	peer.UpdateLastSent()
}
