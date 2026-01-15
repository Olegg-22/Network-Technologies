package screens

import (
	"strconv"

	"lab4/internal/domain"
	"lab4/internal/ui/graphics/components"
	"lab4/internal/ui/graphics/input"
	"lab4/internal/ui/types"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
)

type ConfigScreen struct {
	ctx types.ScreenContext

	inputPlayerName *components.TextInput
	inputGameName   *components.TextInput
	inputWidth      *components.TextInput
	inputHeight     *components.TextInput
	inputFood       *components.TextInput
	inputDelay      *components.TextInput

	btnCreate *components.Button
	btnBack   *components.Button

	errorMsg string
}

func NewConfigScreen(ctx types.ScreenContext) *ConfigScreen {
	s := &ConfigScreen{
		ctx:             ctx,
		inputPlayerName: components.NewTextInput(0, 0, 300, 35, "Your name"),
		inputGameName:   components.NewTextInput(0, 0, 300, 35, "Game name"),
		inputWidth:      components.NewTextInput(0, 0, 140, 35, "40"),
		inputHeight:     components.NewTextInput(0, 0, 140, 35, "30"),
		inputFood:       components.NewTextInput(0, 0, 140, 35, "1"),
		inputDelay:      components.NewTextInput(0, 0, 140, 35, "200"),
		btnCreate:       components.NewButton(0, 0, 140, 45, "Create"),
		btnBack:         components.NewButton(0, 0, 140, 45, "Back"),
	}

	s.inputPlayerName.Text = "OLEG"
	s.inputGameName.Text = "GameOLEG"
	s.inputWidth.Text = "40"
	s.inputHeight.Text = "30"
	s.inputFood.Text = "25"
	s.inputDelay.Text = "200"

	return s
}

func (s *ConfigScreen) Update() types.UIEvent {
	w, _ := s.ctx.Size()
	centerX := w / 2
	startY := 120

	s.inputPlayerName.SetPosition(centerX-150, startY)
	s.inputGameName.SetPosition(centerX-150, startY+60)
	s.inputWidth.SetPosition(centerX-150, startY+120)
	s.inputHeight.SetPosition(centerX+10, startY+120)
	s.inputFood.SetPosition(centerX-150, startY+180)
	s.inputDelay.SetPosition(centerX+10, startY+180)
	s.btnBack.SetPosition(centerX-150, startY+250)
	s.btnCreate.SetPosition(centerX+10, startY+250)

	s.inputPlayerName.Update()
	s.inputGameName.Update()
	s.inputWidth.Update()
	s.inputHeight.Update()
	s.inputFood.Update()
	s.inputDelay.Update()

	if input.IsTabPressed() {
		s.cycleFocus()
	}

	if s.btnBack.Update() || input.IsEscapePressed() {
		return types.UIEvent{Type: types.UIEventShowMenu}
	}

	if s.btnCreate.Update() || input.IsEnterPressed() {
		return s.createGame()
	}

	return types.UIEvent{Type: types.UIEventNone}
}

func (s *ConfigScreen) cycleFocus() {
	inputs := []*components.TextInput{
		s.inputPlayerName, s.inputGameName,
		s.inputWidth, s.inputHeight,
		s.inputFood, s.inputDelay,
	}

	currentIdx := -1
	for i, inp := range inputs {
		if inp.Focused {
			currentIdx = i
			inp.Focused = false
			break
		}
	}

	nextIdx := (currentIdx + 1) % len(inputs)
	inputs[nextIdx].Focused = true
}

func (s *ConfigScreen) createGame() types.UIEvent {
	playerName := s.inputPlayerName.Text
	if playerName == "" {
		s.errorMsg = "Player name is required"
		return types.UIEvent{Type: types.UIEventNone}
	}

	gameName := s.inputGameName.Text
	if gameName == "" {
		s.errorMsg = "Game name is required"
		return types.UIEvent{Type: types.UIEventNone}
	}

	width, err := strconv.Atoi(s.inputWidth.Text)
	if err != nil || width < 10 || width > 100 {
		s.errorMsg = "Width must be 10-100"
		return types.UIEvent{Type: types.UIEventNone}
	}

	height, err := strconv.Atoi(s.inputHeight.Text)
	if err != nil || height < 10 || height > 100 {
		s.errorMsg = "Height must be 10-100"
		return types.UIEvent{Type: types.UIEventNone}
	}

	food, err := strconv.Atoi(s.inputFood.Text)
	if err != nil || food < 0 || food > 100 {
		s.errorMsg = "Food must be 0-100"
		return types.UIEvent{Type: types.UIEventNone}
	}

	delay, err := strconv.Atoi(s.inputDelay.Text)
	if err != nil || delay < 100 || delay > 3000 {
		s.errorMsg = "Delay must be 100-3000 ms"
		return types.UIEvent{Type: types.UIEventNone}
	}

	config := &domain.GameConfig{
		Width:        int32(width),
		Height:       int32(height),
		FoodStatic:   int32(food),
		StateDelayMs: int32(delay),
	}

	return types.UIEvent{
		Type: types.UIEventCreateGame,
		Payload: types.CreateGameData{
			PlayerName: playerName,
			GameName:   gameName,
			Config:     config,
		},
	}
}

func (s *ConfigScreen) Draw(screen *ebiten.Image) {
	screen.Fill(types.ColorBackground)

	fonts := types.GetFonts()
	w, h := s.ctx.Size()
	centerX := w / 2
	startY := 120

	title := "CREATE GAME"
	bounds := text.BoundString(fonts.Normal, title)
	text.Draw(screen, title, fonts.Normal, (w-bounds.Dx())/2, 60, types.ColorTextHighlight)

	text.Draw(screen, "Player Name:", fonts.Normal, centerX-150, startY-15, types.ColorText)
	text.Draw(screen, "Game Name:", fonts.Normal, centerX-150, startY+45, types.ColorText)
	text.Draw(screen, "Width (10-100):", fonts.Normal, centerX-150, startY+105, types.ColorText)
	text.Draw(screen, "Height (10-100):", fonts.Normal, centerX+10, startY+105, types.ColorText)
	text.Draw(screen, "Food (0-100):", fonts.Normal, centerX-150, startY+165, types.ColorText)
	text.Draw(screen, "Delay ms (100-3000):", fonts.Normal, centerX+10, startY+165, types.ColorText)

	s.inputPlayerName.Draw(screen)
	s.inputGameName.Draw(screen)
	s.inputWidth.Draw(screen)
	s.inputHeight.Draw(screen)
	s.inputFood.Draw(screen)
	s.inputDelay.Draw(screen)

	s.btnBack.Draw(screen)
	s.btnCreate.Draw(screen)

	if s.errorMsg != "" {
		bounds := text.BoundString(fonts.Normal, s.errorMsg)
		text.Draw(screen, s.errorMsg, fonts.Normal, (w-bounds.Dx())/2, startY+320, types.ColorError)
	}

	hint := "Press TAB to switch fields, ENTER to create"
	bounds = text.BoundString(fonts.Small, hint)
	text.Draw(screen, hint, fonts.Small, (w-bounds.Dx())/2, h-30, types.ColorTextDim)
}

func (s *ConfigScreen) OnEnter() {
	s.errorMsg = ""
	s.inputPlayerName.Focused = true
}

func (s *ConfigScreen) OnExit() {}
