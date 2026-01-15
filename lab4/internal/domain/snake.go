package domain

type SnakeState int

const (
	SnakeStateAlive  SnakeState = 0
	SnakeStateZombie SnakeState = 1
)

type Snake struct {
	PlayerID      int32
	Points        []Coord
	State         SnakeState
	HeadDirection Direction
}

func NewSnake(playerID int32, head Coord, tailDirection Direction) *Snake {
	return &Snake{
		PlayerID:      playerID,
		Points:        []Coord{head, tailDirection.Delta()},
		State:         SnakeStateAlive,
		HeadDirection: tailDirection.Opposite(),
	}
}

func (s *Snake) Head() Coord {
	if len(s.Points) == 0 {
		return Coord{}
	}
	return s.Points[0]
}

func (s *Snake) Body(field *Field) []Coord {
	if len(s.Points) == 0 {
		return nil
	}

	result := make([]Coord, 0)
	current := s.Points[0]
	result = append(result, field.Normalize(current))

	for i := 1; i < len(s.Points); i++ {
		offset := s.Points[i]
		dx := sign(offset.X)
		dy := sign(offset.Y)
		steps := abs(offset.X) + abs(offset.Y)

		for j := int32(0); j < steps; j++ {
			current = current.Add(Coord{dx, dy})
			result = append(result, field.Normalize(current))
		}
	}

	return result
}

func (s *Snake) Length(field *Field) int {
	return len(s.Body(field))
}

func (s *Snake) SetDirection(dir Direction) bool {
	if dir.IsOpposite(s.HeadDirection) {
		return false
	}
	s.HeadDirection = dir
	return true
}

func (s *Snake) Move(field *Field, ate bool) {
	if len(s.Points) == 0 {
		return
	}

	newHead := field.Move(s.Points[0], s.HeadDirection)

	oldHead := s.Points[0]
	headOffset := Coord{
		X: oldHead.X - newHead.X,
		Y: oldHead.Y - newHead.Y,
	}

	if headOffset.X == field.Width-1 {
		headOffset.X = -1
	} else if headOffset.X == -(field.Width - 1) {
		headOffset.X = 1
	}
	if headOffset.Y == field.Height-1 {
		headOffset.Y = -1
	} else if headOffset.Y == -(field.Height - 1) {
		headOffset.Y = 1
	}

	newPoints := make([]Coord, 0, len(s.Points)+1)
	newPoints = append(newPoints, newHead)

	if len(s.Points) > 1 {
		firstOffset := s.Points[1]
		if (headOffset.X != 0 && firstOffset.X != 0 && sign(headOffset.X) == sign(firstOffset.X)) ||
			(headOffset.Y != 0 && firstOffset.Y != 0 && sign(headOffset.Y) == sign(firstOffset.Y)) {
			newPoints = append(newPoints, Coord{
				X: headOffset.X + firstOffset.X,
				Y: headOffset.Y + firstOffset.Y,
			})
			newPoints = append(newPoints, s.Points[2:]...)
		} else {
			newPoints = append(newPoints, headOffset)
			newPoints = append(newPoints, s.Points[1:]...)
		}
	} else {
		newPoints = append(newPoints, headOffset)
	}

	if !ate && len(newPoints) > 1 {
		lastIdx := len(newPoints) - 1
		last := newPoints[lastIdx]

		if last.X != 0 {
			if last.X > 0 {
				last.X--
			} else {
				last.X++
			}
		} else if last.Y != 0 {
			if last.Y > 0 {
				last.Y--
			} else {
				last.Y++
			}
		}

		if last.X == 0 && last.Y == 0 {
			newPoints = newPoints[:lastIdx]
		} else {
			newPoints[lastIdx] = last
		}
	}

	s.Points = newPoints
}

func sign(x int32) int32 {
	if x > 0 {
		return 1
	} else if x < 0 {
		return -1
	}
	return 0
}

func abs(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}
