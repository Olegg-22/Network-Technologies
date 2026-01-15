package network

import (
	"lab4/internal/domain"
	pb "lab4/pkg/pb"

	"google.golang.org/protobuf/proto"
)

func BuildPingMsg(seq int64, senderID, receiverID int32) *pb.GameMessage {
	return &pb.GameMessage{
		MsgSeq:     proto.Int64(seq),
		SenderId:   proto.Int32(senderID),
		ReceiverId: proto.Int32(receiverID),
		Type: &pb.GameMessage_Ping{
			Ping: &pb.GameMessage_PingMsg{},
		},
	}
}

func BuildSteerMsg(seq int64, dir domain.Direction, senderID int32) *pb.GameMessage {
	return &pb.GameMessage{
		MsgSeq:   proto.Int64(seq),
		SenderId: proto.Int32(senderID),
		Type: &pb.GameMessage_Steer{
			Steer: &pb.GameMessage_SteerMsg{
				Direction: pb.Direction(dir).Enum(),
			},
		},
	}
}

func BuildAckMsg(seq int64, senderID, receiverID int32) *pb.GameMessage {
	return &pb.GameMessage{
		MsgSeq:     proto.Int64(seq),
		SenderId:   proto.Int32(senderID),
		ReceiverId: proto.Int32(receiverID),
		Type: &pb.GameMessage_Ack{
			Ack: &pb.GameMessage_AckMsg{},
		},
	}
}

func BuildStateMsg(seq int64, state *domain.GameState) *pb.GameMessage {
	return &pb.GameMessage{
		MsgSeq: proto.Int64(seq),
		Type: &pb.GameMessage_State{
			State: &pb.GameMessage_StateMsg{
				State: GameStateToPb(state),
			},
		},
	}
}

func BuildAnnouncementMsg(seq int64, gameName string, state *domain.GameState, canJoin bool) *pb.GameMessage {
	players := &pb.GamePlayers{
		Players: make([]*pb.GamePlayer, 0),
	}

	for _, p := range state.GetPlayersSlice() {
		if p.Role != domain.RoleViewer {
			players.Players = append(players.Players, PlayerToPb(p))
		}
	}

	return &pb.GameMessage{
		MsgSeq: proto.Int64(seq),
		Type: &pb.GameMessage_Announcement{
			Announcement: &pb.GameMessage_AnnouncementMsg{
				Games: []*pb.GameAnnouncement{
					{
						Players:  players,
						Config:   GameConfigToPb(state.Config),
						CanJoin:  proto.Bool(canJoin),
						GameName: proto.String(gameName),
					},
				},
			},
		},
	}
}

func BuildJoinMsg(seq int64, playerName, gameName string, role domain.NodeRole) *pb.GameMessage {
	return &pb.GameMessage{
		MsgSeq: proto.Int64(seq),
		Type: &pb.GameMessage_Join{
			Join: &pb.GameMessage_JoinMsg{
				PlayerType:    pb.PlayerType_HUMAN.Enum(),
				PlayerName:    proto.String(playerName),
				GameName:      proto.String(gameName),
				RequestedRole: pb.NodeRole(role).Enum(),
			},
		},
	}
}

func BuildErrorMsg(seq int64, errorMessage string) *pb.GameMessage {
	return &pb.GameMessage{
		MsgSeq: proto.Int64(seq),
		Type: &pb.GameMessage_Error{
			Error: &pb.GameMessage_ErrorMsg{
				ErrorMessage: proto.String(errorMessage),
			},
		},
	}
}

func BuildRoleChangeMsg(seq int64, senderID, receiverID int32, senderRole, receiverRole *domain.NodeRole) *pb.GameMessage {
	msg := &pb.GameMessage{
		MsgSeq:     proto.Int64(seq),
		SenderId:   proto.Int32(senderID),
		ReceiverId: proto.Int32(receiverID),
		Type: &pb.GameMessage_RoleChange{
			RoleChange: &pb.GameMessage_RoleChangeMsg{},
		},
	}

	rc := msg.GetRoleChange()
	if senderRole != nil {
		rc.SenderRole = pb.NodeRole(*senderRole).Enum()
	}
	if receiverRole != nil {
		rc.ReceiverRole = pb.NodeRole(*receiverRole).Enum()
	}

	return msg
}

func GameStateToPb(gs *domain.GameState) *pb.GameState {
	snakes := make([]*pb.GameState_Snake, 0)
	for _, s := range gs.GetSnakesSlice() {
		snakes = append(snakes, SnakeToPb(s))
	}

	foods := make([]*pb.GameState_Coord, 0)
	for _, f := range gs.Foods {
		foods = append(foods, &pb.GameState_Coord{
			X: proto.Int32(f.X),
			Y: proto.Int32(f.Y),
		})
	}

	players := &pb.GamePlayers{
		Players: make([]*pb.GamePlayer, 0),
	}
	for _, p := range gs.GetPlayersSlice() {
		players.Players = append(players.Players, PlayerToPb(p))
	}

	return &pb.GameState{
		StateOrder: proto.Int32(gs.StateOrder),
		Snakes:     snakes,
		Foods:      foods,
		Players:    players,
	}
}

func SnakeToPb(s *domain.Snake) *pb.GameState_Snake {
	points := make([]*pb.GameState_Coord, 0, len(s.Points))
	for _, p := range s.Points {
		points = append(points, &pb.GameState_Coord{
			X: proto.Int32(p.X),
			Y: proto.Int32(p.Y),
		})
	}

	return &pb.GameState_Snake{
		PlayerId:      proto.Int32(s.PlayerID),
		Points:        points,
		State:         pb.GameState_Snake_SnakeState(s.State).Enum(),
		HeadDirection: pb.Direction(s.HeadDirection).Enum(),
	}
}

func PlayerToPb(p *domain.Player) *pb.GamePlayer {
	player := &pb.GamePlayer{
		Name:  proto.String(p.Name),
		Id:    proto.Int32(p.ID),
		Role:  pb.NodeRole(p.Role).Enum(),
		Type:  pb.PlayerType(p.Type).Enum(),
		Score: proto.Int32(p.Score),
	}

	if p.IPAddress != "" {
		player.IpAddress = proto.String(p.IPAddress)
	}
	if p.Port != 0 {
		player.Port = proto.Int32(p.Port)
	}

	return player
}

func GameConfigToPb(c *domain.GameConfig) *pb.GameConfig {
	return &pb.GameConfig{
		Width:        proto.Int32(c.Width),
		Height:       proto.Int32(c.Height),
		FoodStatic:   proto.Int32(c.FoodStatic),
		StateDelayMs: proto.Int32(c.StateDelayMs),
	}
}
