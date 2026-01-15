package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"lab4/internal/app"
	"lab4/internal/domain"
	"lab4/internal/node"
	"lab4/internal/ui/graphics"
	"lab4/internal/ui/graphics/screens"
	"lab4/internal/ui/types"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	debugNetwork()

	application, err := app.NewApp()
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := application.Start(ctx); err != nil {
		log.Fatalf("Failed to start app: %v", err)
	}

	engine := graphics.NewEngine()

	engine.RegisterScreens(
		screens.NewMenuScreen(engine),
		screens.NewConfigScreen(engine),
		screens.NewLobbyScreen(engine),
		screens.NewGameScreen(engine),
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		application.Stop()
		cancel()
		os.Exit(0)
	}()

	go handleAppEvents(application, engine)
	go handleUIEvents(application, engine)

	if err := engine.Run(); err != nil {
		log.Fatalf("UI error: %v", err)
	}

	application.Stop()
}

func handleAppEvents(application *app.App, engine *graphics.Engine) {
	for event := range application.Events() {
		switch event.Type {
		case app.AppEventStateUpdated:
			state := application.GetState()
			if state != nil {
				engine.SetState(state, application.GetMyID(), application.GetCurrentRole())
				//log.Printf("State updated: snakes=%d, food=%d", len(state.Snakes), len(state.Foods))
			}

		case app.AppEventGamesUpdated:
			games := application.GetGames()
			engine.SetGames(games)

		case app.AppEventJoinSuccess:
			engine.SetScreen(types.ScreenGame)
			engine.SetMessage("Joined successfully!")

		case app.AppEventJoinFailed:
			if payload, ok := event.Payload.(node.ErrorPayload); ok {
				engine.SetError(payload.Message)
			}

		case app.AppEventGameOver:
			log.Println("Received GameOver, switching to menu")
			engine.SetScreen(types.ScreenMenu)
			engine.SetMessage("Game over")

		case app.AppEventError:
			if payload, ok := event.Payload.(node.ErrorPayload); ok {
				engine.SetError(payload.Message)
			}
		}
	}
}

func handleUIEvents(application *app.App, engine *graphics.Engine) {
	for event := range engine.Events() {
		switch event.Type {
		case types.UIEventCreateGame:
			data := event.Payload.(types.CreateGameData)
			err := application.CreateGame(app.CreateGameParams{
				PlayerName: data.PlayerName,
				GameName:   data.GameName,
				Config:     data.Config,
			})
			if err != nil {
				log.Printf("Failed to create game: %v", err)
				engine.SetError(err.Error())
			} else {
				engine.SetScreen(types.ScreenGame)
				state := application.GetState()
				if state != nil {
					engine.SetState(state, application.GetMyID(), application.GetCurrentRole())
					log.Printf("Game created: snakes=%d, players=%d", len(state.Snakes), len(state.Players))
				}
			}

		case types.UIEventJoinGame:
			data := event.Payload.(types.JoinGameData)
			games := application.GetGames()
			if data.GameIndex < 0 || data.GameIndex >= len(games) {
				engine.SetError("Invalid game selection")
				continue
			}

			game := games[data.GameIndex]
			err := application.JoinGame(app.JoinGameParams{
				MasterAddr: game.MasterAddr,
				PlayerName: data.PlayerName,
				GameName:   game.Name,
				Config: &domain.GameConfig{
					Width:        game.Config.GetWidth(),
					Height:       game.Config.GetHeight(),
					FoodStatic:   game.Config.GetFoodStatic(),
					StateDelayMs: game.Config.GetStateDelayMs(),
				},
				AsViewer: data.AsViewer,
			})
			if err != nil {
				log.Printf("Failed to join game: %v", err)
				engine.SetError(err.Error())
			} else {
				log.Printf("Join successful, switching to game screen")
				engine.SetScreen(types.ScreenGame)
				engine.SetMessage("Joined successfully!")
			}

		case types.UIEventSteer:
			data := event.Payload.(types.SteerData)
			//log.Printf("Main: sending steer %v", data.Direction)
			if err := application.SendSteer(data.Direction); err != nil {
				log.Printf("Main: failed to send steer: %v", err)
			}

		case types.UIEventExitGame:
			application.ExitGame()
			engine.SetScreen(types.ScreenMenu)

		case types.UIEventQuit:
			application.Stop()
			os.Exit(0)
		}
	}
}

func debugNetwork() {
	log.Println("=== Network Debug ===")

	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Failed to get interfaces: %v", err)
		return
	}

	for _, ifi := range interfaces {
		if ifi.Flags&net.FlagUp == 0 {
			continue
		}

		flags := []string{}
		if ifi.Flags&net.FlagLoopback != 0 {
			flags = append(flags, "loopback")
		}
		if ifi.Flags&net.FlagMulticast != 0 {
			flags = append(flags, "multicast")
		}

		addrs, _ := ifi.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				log.Printf("Interface %s: %s %v", ifi.Name, ipnet.IP, flags)
			}
		}
	}
	log.Println("=== End Debug ===")
}
