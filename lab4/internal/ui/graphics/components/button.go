package components

import (
	"image/color"

	"lab4/internal/ui/types"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Button struct {
	X, Y          int
	Width, Height int
	Text          string
	Enabled       bool
	hovered       bool
	pressed       bool
}

func NewButton(x, y, width, height int, buttonText string) *Button {
	return &Button{
		X:       x,
		Y:       y,
		Width:   width,
		Height:  height,
		Text:    buttonText,
		Enabled: true,
	}
}

func (b *Button) Update() bool {
	if !b.Enabled {
		return false
	}

	mx, my := ebiten.CursorPosition()
	b.hovered = mx >= b.X && mx < b.X+b.Width && my >= b.Y && my < b.Y+b.Height

	wasPressed := b.pressed
	b.pressed = b.hovered && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)

	return wasPressed && !b.pressed && b.hovered
}

func (b *Button) Draw(screen *ebiten.Image) {
	var bgColor color.RGBA
	if !b.Enabled {
		bgColor = types.Darken(types.ColorButton, 0.5)
	} else if b.pressed {
		bgColor = types.Darken(types.ColorButtonHover, 0.8)
	} else if b.hovered {
		bgColor = types.ColorButtonHover
	} else {
		bgColor = types.ColorButton
	}

	vector.DrawFilledRect(screen,
		float32(b.X), float32(b.Y),
		float32(b.Width), float32(b.Height),
		bgColor, false)

	vector.StrokeRect(screen,
		float32(b.X), float32(b.Y),
		float32(b.Width), float32(b.Height),
		1, types.ColorInputBorder, false)

	fonts := types.GetFonts()
	textColor := types.ColorButtonText
	if !b.Enabled {
		textColor = types.ColorTextDim
	}

	bounds := text.BoundString(fonts.Normal, b.Text)
	textX := b.X + (b.Width-bounds.Dx())/2
	textY := b.Y + (b.Height+bounds.Dy())/2

	text.Draw(screen, b.Text, fonts.Normal, textX, textY, textColor)
}

func (b *Button) SetPosition(x, y int) {
	b.X = x
	b.Y = y
}
