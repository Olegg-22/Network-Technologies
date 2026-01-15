package types

import (
	"github.com/hajimehoshi/ebiten/v2"
)

type ScreenType int

const (
	ScreenMenu ScreenType = iota
	ScreenLobby
	ScreenConfig
	ScreenGame
)

type Screen interface {
	Update() UIEvent
	Draw(screen *ebiten.Image)
	OnEnter()
	OnExit()
}

type ScreenContext interface {
	Size() (int, int)
}
