package domain

type Field struct {
	Width  int32
	Height int32
}

func NewField(width, height int32) *Field {
	return &Field{
		Width:  width,
		Height: height,
	}
}

func (f *Field) Normalize(c Coord) Coord {
	x := c.X % f.Width
	if x < 0 {
		x += f.Width
	}
	y := c.Y % f.Height
	if y < 0 {
		y += f.Height
	}
	return Coord{X: x, Y: y}
}

func (f *Field) Move(c Coord, d Direction) Coord {
	return f.Normalize(c.Add(d.Delta()))
}
