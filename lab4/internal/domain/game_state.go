package domain

import (
	"math/rand"
	"sync"
)

type GameState struct {
	StateOrder int32
	Field      *Field
	Config     *GameConfig
	Snakes     map[int32]*Snake
	Foods      []Coord
	Players    map[int32]*Player

	mu sync.RWMutex
}

func NewGameState(config *GameConfig) *GameState {
	return &GameState{
		StateOrder: 0,
		Field:      NewField(config.Width, config.Height),
		Config:     config.Copy(),
		Snakes:     make(map[int32]*Snake),
		Foods:      make([]Coord, 0),
		Players:    make(map[int32]*Player),
	}
}

func (gs *GameState) Copy() *GameState {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	newState := &GameState{
		StateOrder: gs.StateOrder,
		Field:      NewField(gs.Field.Width, gs.Field.Height),
		Config:     gs.Config.Copy(),
		Snakes:     make(map[int32]*Snake),
		Foods:      make([]Coord, len(gs.Foods)),
		Players:    make(map[int32]*Player),
	}

	copy(newState.Foods, gs.Foods)

	for id, snake := range gs.Snakes {
		newSnake := &Snake{
			PlayerID:      snake.PlayerID,
			Points:        make([]Coord, len(snake.Points)),
			State:         snake.State,
			HeadDirection: snake.HeadDirection,
		}
		copy(newSnake.Points, snake.Points)
		newState.Snakes[id] = newSnake
	}

	for id, player := range gs.Players {
		newState.Players[id] = player.Copy()
	}

	return newState
}

func (gs *GameState) AddPlayer(player *Player) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.Players[player.ID] = player
}

func (gs *GameState) RemovePlayer(playerID int32) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	delete(gs.Players, playerID)
}

func (gs *GameState) GetPlayer(playerID int32) *Player {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.Players[playerID]
}

func (gs *GameState) GetSnake(playerID int32) *Snake {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.Snakes[playerID]
}

func (gs *GameState) AliveSnakesCount() int {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	count := 0
	for _, snake := range gs.Snakes {
		if snake.State == SnakeStateAlive {
			count++
		}
	}
	return count
}

func (gs *GameState) RequiredFoodCount() int {
	return int(gs.Config.FoodStatic) + gs.AliveSnakesCount()
}

func (gs *GameState) GetOccupiedCells() map[Coord]bool {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	occupied := make(map[Coord]bool)

	for _, snake := range gs.Snakes {
		for _, cell := range snake.Body(gs.Field) {
			occupied[cell] = true
		}
	}

	for _, food := range gs.Foods {
		occupied[food] = true
	}

	return occupied
}

func (gs *GameState) FindSpawnPlace() (Coord, Direction, bool) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	snakeCells := make(map[Coord]bool)
	for _, snake := range gs.Snakes {
		for _, cell := range snake.Body(gs.Field) {
			snakeCells[cell] = true
		}
	}

	foodCells := make(map[Coord]bool)
	for _, food := range gs.Foods {
		foodCells[food] = true
	}

	for attempts := 0; attempts < 100; attempts++ {
		centerX := rand.Int31n(gs.Field.Width)
		centerY := rand.Int31n(gs.Field.Height)

		found := true
		for dx := int32(-2); dx <= 2 && found; dx++ {
			for dy := int32(-2); dy <= 2 && found; dy++ {
				cell := gs.Field.Normalize(Coord{centerX + dx, centerY + dy})
				if snakeCells[cell] {
					found = false
				}
			}
		}

		if found {
			center := Coord{centerX, centerY}
			directions := []Direction{DirectionUp, DirectionDown, DirectionLeft, DirectionRight}
			tailDir := directions[rand.Intn(4)]
			tailPos := gs.Field.Move(center, tailDir)

			if !foodCells[center] && !foodCells[tailPos] {
				return center, tailDir, true
			}
		}
	}

	return Coord{}, DirectionUp, false
}

func (gs *GameState) SpawnSnake(playerID int32) bool {
	head, tailDir, found := gs.FindSpawnPlace()
	if !found {
		return false
	}

	gs.mu.Lock()
	defer gs.mu.Unlock()

	snake := NewSnake(playerID, head, tailDir)
	gs.Snakes[playerID] = snake
	return true
}

func (gs *GameState) AddFood() bool {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	occupied := make(map[Coord]bool)
	for _, snake := range gs.Snakes {
		for _, cell := range snake.Body(gs.Field) {
			occupied[cell] = true
		}
	}
	for _, food := range gs.Foods {
		occupied[food] = true
	}

	for attempts := 0; attempts < 100; attempts++ {
		x := rand.Int31n(gs.Field.Width)
		y := rand.Int31n(gs.Field.Height)
		pos := Coord{x, y}

		if !occupied[pos] {
			gs.Foods = append(gs.Foods, pos)
			return true
		}
	}

	return false
}

func (gs *GameState) EnsureFood() {
	required := gs.RequiredFoodCount()
	for len(gs.Foods) < required {
		if !gs.AddFood() {
			break
		}
	}
}

func (gs *GameState) RemoveFood(pos Coord) bool {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	for i, food := range gs.Foods {
		if food.Equals(pos) {
			gs.Foods = append(gs.Foods[:i], gs.Foods[i+1:]...)
			return true
		}
	}
	return false
}

func (gs *GameState) HasFood(pos Coord) bool {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	for _, food := range gs.Foods {
		if food.Equals(pos) {
			return true
		}
	}
	return false
}

func (gs *GameState) GeneratePlayerID() int32 {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	maxID := int32(0)
	for id := range gs.Players {
		if id > maxID {
			maxID = id
		}
	}
	return maxID + 1
}

func (gs *GameState) GetPlayersSlice() []*Player {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	result := make([]*Player, 0, len(gs.Players))
	for _, p := range gs.Players {
		result = append(result, p.Copy())
	}
	return result
}

func (gs *GameState) GetSnakesSlice() []*Snake {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	result := make([]*Snake, 0, len(gs.Snakes))
	for _, s := range gs.Snakes {
		result = append(result, s)
	}
	return result
}
