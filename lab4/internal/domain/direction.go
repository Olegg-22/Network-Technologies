package domain

type Direction int32

const (
	DirectionUp    Direction = 1
	DirectionDown  Direction = 2
	DirectionLeft  Direction = 3
	DirectionRight Direction = 4
)

func (d Direction) Opposite() Direction {
	switch d {
	case DirectionUp:
		return DirectionDown
	case DirectionDown:
		return DirectionUp
	case DirectionLeft:
		return DirectionRight
	case DirectionRight:
		return DirectionLeft
	}
	return DirectionUp
}

func (d Direction) Delta() Coord {
	switch d {
	case DirectionUp:
		return Coord{0, -1}
	case DirectionDown:
		return Coord{0, 1}
	case DirectionLeft:
		return Coord{-1, 0}
	case DirectionRight:
		return Coord{1, 0}
	}
	return Coord{}
}

func (d Direction) IsOpposite(other Direction) bool {
	return d.Opposite() == other
}
