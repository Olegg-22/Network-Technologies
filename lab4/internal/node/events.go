package node

import (
	"lab4/internal/domain"
	"net"
)

type EventType int

const (
	EventStateUpdated EventType = iota
	EventPlayerJoined
	EventPlayerLeft
	EventRoleChanged
	EventGameOver
	EventError
	EventJoinSuccess
	EventJoinFailed
)

type Event struct {
	Type    EventType
	Payload interface{}
}

type JoinSuccessPayload struct {
	PlayerID int32
	State    *domain.GameState
}

type ErrorPayload struct {
	Message string
}

type RoleTransitionData struct {
	NewRole    domain.NodeRole
	State      *domain.GameState
	MasterAddr *net.UDPAddr
}

type DeputyTransitionData = RoleTransitionData

type NormalTransitionData = RoleTransitionData
