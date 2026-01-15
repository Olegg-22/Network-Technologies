package domain

type NodeRole int

const (
	RoleNormal NodeRole = 0
	RoleMaster NodeRole = 1
	RoleDeputy NodeRole = 2
	RoleViewer NodeRole = 3
)

type PlayerType int

const (
	PlayerTypeHuman PlayerType = 0
	PlayerTypeRobot PlayerType = 1
)

type Player struct {
	ID        int32
	Name      string
	IPAddress string
	Port      int32
	Role      NodeRole
	Type      PlayerType
	Score     int32
}

func NewPlayer(id int32, name string, role NodeRole) *Player {
	return &Player{
		ID:    id,
		Name:  name,
		Role:  role,
		Type:  PlayerTypeHuman,
		Score: 0,
	}
}

func (p *Player) Copy() *Player {
	return &Player{
		ID:        p.ID,
		Name:      p.Name,
		IPAddress: p.IPAddress,
		Port:      p.Port,
		Role:      p.Role,
		Type:      p.Type,
		Score:     p.Score,
	}
}
