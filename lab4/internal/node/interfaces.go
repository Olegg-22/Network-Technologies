package node

import (
	"context"
	"net"

	"lab4/internal/domain"
	pb "lab4/pkg/pb"
)

type Role interface {
	Start(ctx context.Context) error
	Stop() error
	HandleMessage(msg *pb.GameMessage, from *net.UDPAddr)
	GetRole() domain.NodeRole
	GetState() *domain.GameState
	SendSteer(dir domain.Direction) error
}
