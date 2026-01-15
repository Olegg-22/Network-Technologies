package screens

import (
	"fmt"

	"lab4/internal/app"
	"lab4/internal/ui/graphics/components"
	"lab4/internal/ui/graphics/input"
	"lab4/internal/ui/types"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type LobbyScreen struct {
	ctx types.ScreenContext

	games         []app.GameInfo
	selectedIndex int

	inputPlayerName *components.TextInput
	btnJoin         *components.Button
	btnWatch        *components.Button
	btnBack         *components.Button

	errorMsg string
	message  string
}

func NewLobbyScreen(ctx types.ScreenContext) *LobbyScreen {
	return &LobbyScreen{
		ctx:             ctx,
		selectedIndex:   -1,
		inputPlayerName: components.NewTextInput(0, 0, 200, 35, "Your name"),
		btnJoin:         components.NewButton(0, 0, 120, 40, "Join"),
		btnWatch:        components.NewButton(0, 0, 120, 40, "Watch"),
		btnBack:         components.NewButton(0, 0, 120, 40, "Back"),
	}
}

func (s *LobbyScreen) SetGames(games []app.GameInfo) {
	s.games = games
	if s.selectedIndex >= len(games) {
		s.selectedIndex = len(games) - 1
	}
}

func (s *LobbyScreen) Update() types.UIEvent {
	w, h := s.ctx.Size()

	s.inputPlayerName.SetPosition(50, h-80)
	if s.inputPlayerName.Text == "" {
		s.inputPlayerName.Text = "Player"
	}
	s.btnJoin.SetPosition(270, h-85)
	s.btnWatch.SetPosition(400, h-85)
	s.btnBack.SetPosition(w-140, h-85)

	s.inputPlayerName.Update()

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if mx >= 50 && mx < w-50 && my >= 100 && my < h-120 {
			idx := (my - 100) / 50
			if idx >= 0 && idx < len(s.games) {
				s.selectedIndex = idx
			}
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyUp) && s.selectedIndex > 0 {
		s.selectedIndex--
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) && s.selectedIndex < len(s.games)-1 {
		s.selectedIndex++
	}

	canJoin := s.selectedIndex >= 0 && s.selectedIndex < len(s.games)
	if canJoin {
		s.btnJoin.Enabled = s.games[s.selectedIndex].CanJoin
	} else {
		s.btnJoin.Enabled = false
	}
	s.btnWatch.Enabled = canJoin

	if s.btnBack.Update() || input.IsEscapePressed() {
		return types.UIEvent{Type: types.UIEventShowMenu}
	}

	if s.btnJoin.Update() || (input.IsEnterPressed() && s.btnJoin.Enabled) {
		return s.joinGame(false)
	}

	if s.btnWatch.Update() {
		return s.joinGame(true)
	}

	return types.UIEvent{Type: types.UIEventNone}
}

func (s *LobbyScreen) joinGame(asViewer bool) types.UIEvent {
	if s.selectedIndex < 0 || s.selectedIndex >= len(s.games) {
		s.errorMsg = "Select a game first"
		return types.UIEvent{Type: types.UIEventNone}
	}

	playerName := s.inputPlayerName.Text
	if playerName == "" {
		playerName = "Player"
	}

	return types.UIEvent{
		Type: types.UIEventJoinGame,
		Payload: types.JoinGameData{
			GameIndex:  s.selectedIndex,
			PlayerName: playerName,
			AsViewer:   asViewer,
		},
	}
}

func (s *LobbyScreen) Draw(screen *ebiten.Image) {
	screen.Fill(types.ColorBackground)

	fonts := types.GetFonts()
	w, h := s.ctx.Size()

	title := "AVAILABLE GAMES"
	bounds := text.BoundString(fonts.Normal, title)
	text.Draw(screen, title, fonts.Normal, (w-bounds.Dx())/2, 50, types.ColorTextHighlight)

	if len(s.games) == 0 {
		msg := "No games available. Waiting for announcements..."
		bounds := text.BoundString(fonts.Normal, msg)
		text.Draw(screen, msg, fonts.Normal, (w-bounds.Dx())/2, 200, types.ColorTextDim)
	} else {
		for i, game := range s.games {
			y := 100 + i*50

			if i == s.selectedIndex {
				vector.DrawFilledRect(screen, 50, float32(y-5), float32(w-100), 45, types.ColorButton, false)
			}

			status := "●"
			statusColor := types.ColorSuccess
			if !game.CanJoin {
				statusColor = types.ColorError
			}
			text.Draw(screen, status, fonts.Normal, 60, y+20, statusColor)

			gameInfo := fmt.Sprintf("%s  |  Players: %d  |  %dx%d  |  %dms",
				game.Name, game.Players,
				game.Config.GetWidth(), game.Config.GetHeight(),
				game.Config.GetStateDelayMs())
			text.Draw(screen, gameInfo, fonts.Normal, 85, y+20, types.ColorText)

			addrInfo := fmt.Sprintf("%s", game.MasterAddr)
			text.Draw(screen, addrInfo, fonts.Normal, w-200, y+20, types.ColorTextDim)
		}
	}

	vector.DrawFilledRect(screen, 0, float32(h-110), float32(w), 110, types.Darken(types.ColorBackground, 0.8), false)
	vector.StrokeLine(screen, 0, float32(h-110), float32(w), float32(h-110), 1, types.ColorGrid, false)

	text.Draw(screen, "Name:", fonts.Normal, 50, h-55, types.ColorText)
	s.inputPlayerName.Draw(screen)
	s.btnJoin.Draw(screen)
	s.btnWatch.Draw(screen)
	s.btnBack.Draw(screen)

	if s.errorMsg != "" {
		text.Draw(screen, s.errorMsg, fonts.Normal, 50, h-25, types.ColorError)
	}

	if s.message != "" {
		text.Draw(screen, s.message, fonts.Normal, 50, h-25, types.ColorSuccess)
	}

	hint := "Use UP/DOWN to select, ENTER to join"
	text.Draw(screen, hint, fonts.Small, w-250, h-25, types.ColorTextDim)
}

func (s *LobbyScreen) OnEnter() {
	s.errorMsg = ""
	s.message = ""
	s.inputPlayerName.Text = "OLEG"
}

func (s *LobbyScreen) OnExit() {}

func (s *LobbyScreen) SetError(err string) {
	s.errorMsg = err
	s.message = ""
}

func (s *LobbyScreen) SetMessage(msg string) {
	s.message = msg
	s.errorMsg = ""
}
