package screens

import (
	"lab4/internal/ui/graphics/components"
	"lab4/internal/ui/graphics/input"
	"lab4/internal/ui/types"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
)

type MenuScreen struct {
	ctx types.ScreenContext

	btnCreate *components.Button
	btnJoin   *components.Button
	btnQuit   *components.Button
}

func NewMenuScreen(ctx types.ScreenContext) *MenuScreen {
	return &MenuScreen{
		ctx:       ctx,
		btnCreate: components.NewButton(0, 0, 250, 50, "Create Game"),
		btnJoin:   components.NewButton(0, 0, 250, 50, "Join Game"),
		btnQuit:   components.NewButton(0, 0, 250, 50, "Quit"),
	}
}

func (s *MenuScreen) Update() types.UIEvent {
	w, h := s.ctx.Size()
	centerX := w / 2
	centerY := h / 2

	s.btnCreate.SetPosition(centerX-125, centerY-80)
	s.btnJoin.SetPosition(centerX-125, centerY-20)
	s.btnQuit.SetPosition(centerX-125, centerY+40)

	if s.btnCreate.Update() {
		return types.UIEvent{Type: types.UIEventShowConfig}
	}

	if s.btnJoin.Update() {
		return types.UIEvent{Type: types.UIEventShowLobby}
	}

	if s.btnQuit.Update() || input.IsEscapePressed() {
		return types.UIEvent{Type: types.UIEventQuit}
	}

	return types.UIEvent{Type: types.UIEventNone}
}

func (s *MenuScreen) Draw(screen *ebiten.Image) {
	screen.Fill(types.ColorBackground)

	fonts := types.GetFonts()
	w, h := s.ctx.Size()

	title := "SNAKE ONLINE"
	bounds := text.BoundString(fonts.Normal, title)
	x := (w - bounds.Dx()) / 2

	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			text.Draw(screen, title, fonts.Normal, x+dx, 100+dy, types.ColorTextHighlight)
		}
	}
	text.Draw(screen, title, fonts.Normal, x, 100, types.ColorTextHighlight)

	subtitle := "Multiplayer Snake Game"
	bounds = text.BoundString(fonts.Normal, subtitle)
	x = (w - bounds.Dx()) / 2
	text.Draw(screen, subtitle, fonts.Normal, x, 130, types.ColorTextDim)

	s.btnCreate.Draw(screen)
	s.btnJoin.Draw(screen)
	s.btnQuit.Draw(screen)

	hint := "Press ESC to quit"
	bounds = text.BoundString(fonts.Small, hint)
	x = (w - bounds.Dx()) / 2
	text.Draw(screen, hint, fonts.Small, x, h-30, types.ColorTextDim)
}

func (s *MenuScreen) OnEnter() {}

func (s *MenuScreen) OnExit() {}
