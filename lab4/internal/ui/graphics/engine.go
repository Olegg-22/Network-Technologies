package graphics

import (
	"log"
	"sync"

	"lab4/internal/app"
	"lab4/internal/domain"
	"lab4/internal/ui/types"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	DefaultWidth  = 1024
	DefaultHeight = 768
)

type Engine struct {
	width  int
	height int

	currentScreen types.ScreenType
	screenMap     map[types.ScreenType]types.Screen

	state *domain.GameState
	myID  int32
	role  domain.NodeRole
	games []app.GameInfo

	dataMu sync.RWMutex

	eventCh chan types.UIEvent
}

func NewEngine() *Engine {
	types.InitFonts()

	e := &Engine{
		width:         DefaultWidth,
		height:        DefaultHeight,
		currentScreen: types.ScreenMenu,
		screenMap:     make(map[types.ScreenType]types.Screen),
		eventCh:       make(chan types.UIEvent, 100),
	}

	return e
}

func (e *Engine) RegisterScreens(
	menu types.Screen,
	config types.Screen,
	lobby types.Screen,
	game types.Screen,
) {
	e.screenMap[types.ScreenMenu] = menu
	e.screenMap[types.ScreenConfig] = config
	e.screenMap[types.ScreenLobby] = lobby
	e.screenMap[types.ScreenGame] = game
}

func (e *Engine) Run() error {
	ebiten.SetWindowSize(e.width, e.height)
	ebiten.SetWindowTitle("Snake Online")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	return ebiten.RunGame(e)
}

func (e *Engine) Update() error {
	e.width, e.height = ebiten.WindowSize()

	screen := e.screenMap[e.currentScreen]
	if screen == nil {
		return nil
	}
	event := screen.Update()

	e.handleEvent(event)

	return nil
}

func (e *Engine) Draw(screen *ebiten.Image) {
	currentScreen := e.screenMap[e.currentScreen]
	if currentScreen == nil {
		return
	}

	if updater, ok := currentScreen.(GameStateUpdater); ok {
		e.dataMu.RLock()
		updater.SetState(e.state, e.myID, e.role)
		e.dataMu.RUnlock()
	}

	if updater, ok := currentScreen.(GamesUpdater); ok {
		e.dataMu.RLock()
		updater.SetGames(e.games)
		e.dataMu.RUnlock()
	}

	currentScreen.Draw(screen)
}

func (e *Engine) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

func (e *Engine) Size() (int, int) {
	return e.width, e.height
}

func (e *Engine) Events() <-chan types.UIEvent {
	return e.eventCh
}

func (e *Engine) SetScreen(screen types.ScreenType) {
	if e.currentScreen != screen {
		if s := e.screenMap[e.currentScreen]; s != nil {
			s.OnExit()
		}
		e.currentScreen = screen
		if s := e.screenMap[e.currentScreen]; s != nil {
			s.OnEnter()
		}
	}
}

func (e *Engine) SetState(state *domain.GameState, myID int32, role domain.NodeRole) {
	e.dataMu.Lock()
	e.state = state
	e.myID = myID
	e.role = role
	e.dataMu.Unlock()
}

func (e *Engine) SetGames(games []app.GameInfo) {
	e.dataMu.Lock()
	e.games = games
	e.dataMu.Unlock()
}

func (e *Engine) SetError(err string) {
	if s, ok := e.screenMap[e.currentScreen].(ErrorSetter); ok {
		s.SetError(err)
	}
}

func (e *Engine) SetMessage(msg string) {
	if s, ok := e.screenMap[e.currentScreen].(MessageSetter); ok {
		s.SetMessage(msg)
	}
}

func (e *Engine) handleEvent(event types.UIEvent) {
	switch event.Type {
	case types.UIEventNone:
		return

	case types.UIEventShowMenu:
		e.SetScreen(types.ScreenMenu)

	case types.UIEventShowConfig:
		e.SetScreen(types.ScreenConfig)

	case types.UIEventShowLobby:
		e.SetScreen(types.ScreenLobby)

	case types.UIEventQuit:
		select {
		case e.eventCh <- event:
		default:
		}

	default:
		select {
		case e.eventCh <- event:
		default:
			log.Println("Event channel full, dropping event")
		}
	}
}

func (e *Engine) GetCurrentScreen() types.ScreenType {
	return e.currentScreen
}

type GameStateUpdater interface {
	SetState(state *domain.GameState, myID int32, role domain.NodeRole)
}

type GamesUpdater interface {
	SetGames(games []app.GameInfo)
}

type ErrorSetter interface {
	SetError(err string)
}

type MessageSetter interface {
	SetMessage(msg string)
}
