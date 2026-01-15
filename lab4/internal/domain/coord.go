package domain

type Coord struct {
	X int32
	Y int32
}

func (c Coord) Add(other Coord) Coord {
	return Coord{
		X: c.X + other.X,
		Y: c.Y + other.Y,
	}
}

func (c Coord) Equals(other Coord) bool {
	return c.X == other.X && c.Y == other.Y
}
