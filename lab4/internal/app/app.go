package app

import (
	"context"
	"fmt"
	"lab4/internal/domain"
	"lab4/internal/network"
	"lab4/internal/node"
	"log"
	"net"
	"sync"
)

type App struct {
	socket       *network.Socket
	multicast    *network.Multicast
	stateMachine *node.StateMachine
	discovery    *DiscoveryService

	nodeEventCh chan node.Event

	eventCh chan AppEvent
	inputCh chan InputEvent

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type AppEvent struct {
	Type    AppEventType
	Payload interface{}
}

type AppEventType int

const (
	AppEventStateUpdated AppEventType = iota
	AppEventGamesUpdated
	AppEventJoinSuccess
	AppEventJoinFailed
	AppEventGameOver
	AppEventError
)

type InputEvent struct {
	Type    InputEventType
	Payload interface{}
}

type InputEventType int

const (
	InputCreateGame InputEventType = iota
	InputJoinGame
	InputExitGame
	InputSteer
	InputQuit
)

type CreateGameParams struct {
	PlayerName string
	GameName   string
	Config     *domain.GameConfig
}

type JoinGameParams struct {
	MasterAddr *net.UDPAddr
	PlayerName string
	GameName   string
	Config     *domain.GameConfig
	AsViewer   bool
}

func NewApp() (*App, error) {

	socket, err := network.NewSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}

	multicast, err := network.NewMulticast()
	if err != nil {
		socket.Close()
		return nil, fmt.Errorf("failed to create multicast: %w", err)
	}

	nodeEventCh := make(chan node.Event, 100)
	stateMachine := node.NewStateMachine(socket, multicast, nodeEventCh)

	return &App{
		socket:       socket,
		multicast:    multicast,
		stateMachine: stateMachine,
		discovery:    NewDiscoveryService(multicast),
		nodeEventCh:  nodeEventCh,
		eventCh:      make(chan AppEvent, 100),
		inputCh:      make(chan InputEvent, 100),
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	a.ctx, a.cancel = context.WithCancel(ctx)

	a.stateMachine.Start(a.ctx)

	a.discovery.Start(a.ctx)

	a.wg.Add(1)
	go a.eventLoop()

	a.wg.Add(1)
	go a.inputLoop()

	log.Printf("App started on port %d", a.socket.LocalPort())

	return nil
}

func (a *App) Stop() {
	if a.cancel != nil {
		a.cancel()
	}

	a.stateMachine.Stop()
	a.discovery.Stop()
	a.socket.Close()
	a.multicast.Close()

	a.wg.Wait()
}

func (a *App) Events() <-chan AppEvent {
	return a.eventCh
}

func (a *App) Input() chan<- InputEvent {
	return a.inputCh
}

func (a *App) GetState() *domain.GameState {
	return a.stateMachine.GetState()
}

func (a *App) GetGames() []GameInfo {
	return a.discovery.GetGames()
}

func (a *App) GetCurrentRole() domain.NodeRole {
	role := a.stateMachine.GetCurrentRole()
	if role == nil {
		return domain.RoleViewer
	}
	return role.GetRole()
}

func (a *App) GetMyID() int32 {
	return a.stateMachine.GetMyID()
}

func (a *App) CreateGame(params CreateGameParams) error {
	return a.stateMachine.CreateGame(params.PlayerName, params.GameName, params.Config)
}

func (a *App) JoinGame(params JoinGameParams) error {
	return a.stateMachine.JoinGame(
		params.MasterAddr,
		params.PlayerName,
		params.GameName,
		params.Config,
		params.AsViewer,
	)
}

func (a *App) SendSteer(dir domain.Direction) error {
	return a.stateMachine.SendSteer(dir)
}

func (a *App) ExitGame() {
	a.stateMachine.ExitGame()
}

func (a *App) eventLoop() {
	defer a.wg.Done()

	discoveryEvents := a.discovery.Events()

	for {
		select {
		case <-a.ctx.Done():
			return

		case event := <-a.nodeEventCh:
			a.handleNodeEvent(event)

		case event := <-discoveryEvents:
			a.handleDiscoveryEvent(event)
		}
	}
}

func (a *App) handleNodeEvent(event node.Event) {
	switch event.Type {
	case node.EventStateUpdated:
		a.eventCh <- AppEvent{Type: AppEventStateUpdated}

	case node.EventPlayerJoined:
		log.Printf("Player joined: %v", event.Payload)

	case node.EventPlayerLeft:
		log.Printf("Player left: %v", event.Payload)

	case node.EventRoleChanged:
		log.Printf("Role changed to: %v", event.Payload)

	case node.EventGameOver:
		log.Println("Game over!")
		a.eventCh <- AppEvent{Type: AppEventGameOver}

	case node.EventError:
		a.eventCh <- AppEvent{
			Type:    AppEventError,
			Payload: event.Payload,
		}

	case node.EventJoinSuccess:
		a.eventCh <- AppEvent{
			Type:    AppEventJoinSuccess,
			Payload: event.Payload,
		}

	case node.EventJoinFailed:
		a.eventCh <- AppEvent{
			Type:    AppEventJoinFailed,
			Payload: event.Payload,
		}
	}
}

func (a *App) handleDiscoveryEvent(event DiscoveryEvent) {
	switch event.Type {
	case DiscoveryGamesUpdated:
		a.eventCh <- AppEvent{Type: AppEventGamesUpdated}
	}
}

func (a *App) inputLoop() {
	defer a.wg.Done()

	for {
		select {
		case <-a.ctx.Done():
			return

		case input := <-a.inputCh:
			a.handleInput(input)
		}
	}
}

func (a *App) handleInput(input InputEvent) {
	switch input.Type {
	case InputCreateGame:
		params := input.Payload.(CreateGameParams)

		if err := a.CreateGame(params); err != nil {

			log.Printf("Failed to create game: %v", err)

			a.eventCh <- AppEvent{
				Type:    AppEventError,
				Payload: node.ErrorPayload{Message: err.Error()},
			}
		}

	case InputJoinGame:
		params := input.Payload.(JoinGameParams)

		if err := a.JoinGame(params); err != nil {

			log.Printf("Failed to join game: %v", err)
			a.eventCh <- AppEvent{
				Type:    AppEventJoinFailed,
				Payload: node.ErrorPayload{Message: err.Error()},
			}
		} else {
			a.eventCh <- AppEvent{Type: AppEventJoinSuccess}
		}

	case InputExitGame:
		a.ExitGame()

	case InputSteer:
		dir := input.Payload.(domain.Direction)
		a.SendSteer(dir)

	case InputQuit:
		a.Stop()
	}
}
