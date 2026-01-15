package input

import (
	"lab4/internal/domain"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type KeyboardHandler struct{}

func NewKeyboardHandler() *KeyboardHandler {
	return &KeyboardHandler{}
}

func (kh *KeyboardHandler) Update() domain.Direction {
	if inpututil.IsKeyJustPressed(ebiten.KeyW) || inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		return domain.DirectionUp
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) || inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		return domain.DirectionDown
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyA) || inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		return domain.DirectionLeft
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyD) || inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		return domain.DirectionRight
	}

	return 0
}

func IsEscapePressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeyEscape)
}

func IsEnterPressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeyEnter)
}

func IsTabPressed() bool {
	return inpututil.IsKeyJustPressed(ebiten.KeyTab)
}
