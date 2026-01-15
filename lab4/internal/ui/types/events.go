package types

import (
	"lab4/internal/domain"
)

type UIEvent struct {
	Type    UIEventType
	Payload interface{}
}

type UIEventType int

const (
	UIEventNone UIEventType = iota
	UIEventCreateGame
	UIEventJoinGame
	UIEventExitGame
	UIEventSteer
	UIEventQuit
	UIEventShowLobby
	UIEventShowConfig
	UIEventShowMenu
)

type CreateGameData struct {
	PlayerName string
	GameName   string
	Config     *domain.GameConfig
}

type JoinGameData struct {
	GameIndex  int
	PlayerName string
	AsViewer   bool
}

type SteerData struct {
	Direction domain.Direction
}
