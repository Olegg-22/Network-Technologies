package components

import (
	"lab4/internal/domain"
	"lab4/internal/ui/types"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type FieldRenderer struct {
	CellSize int
	OffsetX  int
	OffsetY  int
}

func NewFieldRenderer() *FieldRenderer {
	return &FieldRenderer{
		CellSize: 15,
		OffsetX:  20,
		OffsetY:  20,
	}
}

func (fr *FieldRenderer) CalculateLayout(screenWidth, screenHeight int, field *domain.Field) {
	if field == nil {
		return
	}

	availableWidth := screenWidth - 300
	availableHeight := screenHeight - 80

	cellW := availableWidth / int(field.Width)
	cellH := availableHeight / int(field.Height)

	fr.CellSize = cellW
	if cellH < cellW {
		fr.CellSize = cellH
	}

	if fr.CellSize < 5 {
		fr.CellSize = 5
	}
	if fr.CellSize > 30 {
		fr.CellSize = 30
	}

	fieldWidth := fr.CellSize * int(field.Width)
	fieldHeight := fr.CellSize * int(field.Height)
	fr.OffsetX = (availableWidth-fieldWidth)/2 + 20
	fr.OffsetY = (availableHeight-fieldHeight)/2 + 60
}

func (fr *FieldRenderer) DrawField(screen *ebiten.Image, field *domain.Field) {
	if field == nil {
		return
	}

	w := float32(int(field.Width) * fr.CellSize)
	h := float32(int(field.Height) * fr.CellSize)

	vector.DrawFilledRect(screen,
		float32(fr.OffsetX), float32(fr.OffsetY),
		w, h,
		types.ColorFieldBg, false)

	for x := int32(0); x <= field.Width; x++ {
		x1 := float32(fr.OffsetX + int(x)*fr.CellSize)
		vector.StrokeLine(screen,
			x1, float32(fr.OffsetY),
			x1, float32(fr.OffsetY)+h,
			1, types.ColorGrid, false)
	}
	for y := int32(0); y <= field.Height; y++ {
		y1 := float32(fr.OffsetY + int(y)*fr.CellSize)
		vector.StrokeLine(screen,
			float32(fr.OffsetX), y1,
			float32(fr.OffsetX)+w, y1,
			1, types.ColorGrid, false)
	}
}

func (fr *FieldRenderer) DrawFood(screen *ebiten.Image, foods []domain.Coord) {
	padding := float32(fr.CellSize) / 4

	for _, food := range foods {
		x := float32(fr.OffsetX+int(food.X)*fr.CellSize) + padding
		y := float32(fr.OffsetY+int(food.Y)*fr.CellSize) + padding
		size := float32(fr.CellSize) - padding*2

		vector.DrawFilledRect(screen, x, y, size, size, types.ColorFood, false)
	}
}

func (fr *FieldRenderer) DrawSnake(screen *ebiten.Image, snake *domain.Snake, field *domain.Field, isMe bool) {
	if snake == nil || field == nil {
		return
	}

	body := snake.Body(field)
	baseColor := types.GetSnakeColor(snake.PlayerID)

	for i, cell := range body {
		x := float32(fr.OffsetX + int(cell.X)*fr.CellSize + 1)
		y := float32(fr.OffsetY + int(cell.Y)*fr.CellSize + 1)
		size := float32(fr.CellSize - 2)

		cellColor := baseColor

		if i == 0 {
			cellColor = types.Darken(baseColor, 0.7)
		}

		if snake.State == domain.SnakeStateZombie {
			cellColor = types.Darken(cellColor, 0.5)
		}

		vector.DrawFilledRect(screen, x, y, size, size, cellColor, false)

		if isMe && i == 0 {
			vector.StrokeRect(screen, x-1, y-1, size+2, size+2, 2, types.ColorTextHighlight, false)
		}
	}
}
