package network

import (
	"lab4/internal/domain"
	pb "lab4/pkg/pb"
)

func PbToGameState(pbState *pb.GameState, config *domain.GameConfig) *domain.GameState {
	gs := domain.NewGameState(config)
	gs.StateOrder = pbState.GetStateOrder()

	for _, pbSnake := range pbState.GetSnakes() {
		snake := PbToSnake(pbSnake)
		gs.Snakes[snake.PlayerID] = snake
	}

	for _, pbFood := range pbState.GetFoods() {
		gs.Foods = append(gs.Foods, domain.Coord{
			X: pbFood.GetX(),
			Y: pbFood.GetY(),
		})
	}

	for _, pbPlayer := range pbState.GetPlayers().GetPlayers() {
		player := PbToPlayer(pbPlayer)
		gs.Players[player.ID] = player
	}

	return gs
}

func PbToSnake(pbSnake *pb.GameState_Snake) *domain.Snake {
	points := make([]domain.Coord, 0, len(pbSnake.GetPoints()))
	for _, p := range pbSnake.GetPoints() {
		points = append(points, domain.Coord{
			X: p.GetX(),
			Y: p.GetY(),
		})
	}

	return &domain.Snake{
		PlayerID:      pbSnake.GetPlayerId(),
		Points:        points,
		State:         domain.SnakeState(pbSnake.GetState()),
		HeadDirection: domain.Direction(pbSnake.GetHeadDirection()),
	}
}

func PbToPlayer(pbPlayer *pb.GamePlayer) *domain.Player {
	return &domain.Player{
		ID:        pbPlayer.GetId(),
		Name:      pbPlayer.GetName(),
		IPAddress: pbPlayer.GetIpAddress(),
		Port:      pbPlayer.GetPort(),
		Role:      domain.NodeRole(pbPlayer.GetRole()),
		Type:      domain.PlayerType(pbPlayer.GetType()),
		Score:     pbPlayer.GetScore(),
	}
}

func PbToDirection(pbDir pb.Direction) domain.Direction {
	return domain.Direction(pbDir)
}
