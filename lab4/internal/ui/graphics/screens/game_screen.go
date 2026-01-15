package screens

import (
	"fmt"
	"log"

	"lab4/internal/domain"
	"lab4/internal/ui/graphics/components"
	"lab4/internal/ui/graphics/input"
	"lab4/internal/ui/types"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
)

type GameScreen struct {
	ctx types.ScreenContext

	fieldRenderer *components.FieldRenderer
	scoreboard    *components.Scoreboard
	keyboard      *input.KeyboardHandler

	state *domain.GameState
	myID  int32
	role  domain.NodeRole

	message  string
	errorMsg string
}

func NewGameScreen(ctx types.ScreenContext) *GameScreen {
	return &GameScreen{
		ctx:           ctx,
		fieldRenderer: components.NewFieldRenderer(),
		scoreboard:    components.NewScoreboard(0, 0, 230, 400),
		keyboard:      input.NewKeyboardHandler(),
	}
}

func (s *GameScreen) SetState(state *domain.GameState, myID int32, role domain.NodeRole) {
	s.state = state
	s.myID = myID
	s.role = role
}

func (s *GameScreen) Update() types.UIEvent {
	if input.IsEscapePressed() {
		return types.UIEvent{Type: types.UIEventExitGame}
	}

	dir := s.keyboard.Update()
	if dir != 0 {
		//log.Printf("GameScreen: key pressed, dir=%v, role=%v", dir, s.role)
		if s.role != domain.RoleViewer {
			return types.UIEvent{
				Type:    types.UIEventSteer,
				Payload: types.SteerData{Direction: dir},
			}
		} else {
			log.Printf("GameScreen: ignoring steer because role is VIEWER")
		}
	}

	return types.UIEvent{Type: types.UIEventNone}
}

func (s *GameScreen) Draw(screen *ebiten.Image) {
	screen.Fill(types.ColorBackground)

	w, h := s.ctx.Size()
	fonts := types.GetFonts()

	if s.state == nil {
		msg := "Waiting for game state..."
		bounds := text.BoundString(fonts.Normal, msg)
		text.Draw(screen, msg, fonts.Normal, (w-bounds.Dx())/2, h/2, types.ColorTextDim)
		return
	}

	s.fieldRenderer.CalculateLayout(w, h, s.state.Field)

	s.fieldRenderer.DrawField(screen, s.state.Field)
	s.fieldRenderer.DrawFood(screen, s.state.Foods)

	for _, snake := range s.state.Snakes {
		isMe := snake.PlayerID == s.myID
		s.fieldRenderer.DrawSnake(screen, snake, s.state.Field, isMe)
	}

	s.scoreboard.X = w - 250
	s.scoreboard.Y = 60
	s.scoreboard.Height = h - 120
	s.scoreboard.Draw(screen, s.state.Players, s.myID)

	s.drawHeader(screen, w)
	s.drawFooter(screen, w, h)
}

func (s *GameScreen) drawHeader(screen *ebiten.Image, w int) {
	fonts := types.GetFonts()

	roleText := ""
	switch s.role {
	case domain.RoleMaster:
		roleText = "[MASTER]"
	case domain.RoleDeputy:
		roleText = "[DEPUTY]"
	case domain.RoleNormal:
		roleText = "[NORMAL]"
	case domain.RoleViewer:
		roleText = "[VIEWER]"
	}
	text.Draw(screen, roleText, fonts.Normal, 20, 30, types.ColorTextHighlight)

	if s.state != nil {
		stateInfo := fmt.Sprintf("State: #%d  |  Players: %d  |  Food: %d",
			s.state.StateOrder,
			len(s.state.Players),
			len(s.state.Foods))
		text.Draw(screen, stateInfo, fonts.Normal, 120, 30, types.ColorText)
	}

	if s.state != nil {
		if player := s.state.Players[s.myID]; player != nil {
			scoreText := fmt.Sprintf("Score: %d", player.Score)
			bounds := text.BoundString(fonts.Normal, scoreText)
			text.Draw(screen, scoreText, fonts.Normal, w-bounds.Dx()-20, 30, types.ColorTextHighlight)
		}
	}
}

func (s *GameScreen) drawFooter(screen *ebiten.Image, w, h int) {
	fonts := types.GetFonts()

	hint := "W/A/S/D or Arrows to move  |  ESC to exit"
	if s.role == domain.RoleViewer {
		hint = "Viewer mode (no control)  |  ESC to exit"
	}
	text.Draw(screen, hint, fonts.Small, 20, h-15, types.ColorTextDim)

	if s.errorMsg != "" {
		bounds := text.BoundString(fonts.Normal, s.errorMsg)
		text.Draw(screen, s.errorMsg, fonts.Normal, w-bounds.Dx()-20, h-15, types.ColorError)
	} else if s.message != "" {
		bounds := text.BoundString(fonts.Normal, s.message)
		text.Draw(screen, s.message, fonts.Normal, w-bounds.Dx()-20, h-15, types.ColorSuccess)
	}
}

func (s *GameScreen) OnEnter() {
	s.errorMsg = ""
	s.message = ""
}

func (s *GameScreen) OnExit() {}

func (s *GameScreen) SetError(err string) {
	s.errorMsg = err
}

func (s *GameScreen) SetMessage(msg string) {
	s.message = msg
}
