package domain

import (
	"math/rand"
)

type TickResult struct {
	KilledPlayers []int32
	ScoreChanges  map[int32]int32
}

func (gs *GameState) Tick(moves map[int32]Direction) *TickResult {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	result := &TickResult{
		KilledPlayers: make([]int32, 0),
		ScoreChanges:  make(map[int32]int32),
	}

	gs.StateOrder++

	for playerID, dir := range moves {
		if snake, ok := gs.Snakes[playerID]; ok {
			if snake.State == SnakeStateAlive {
				snake.SetDirection(dir)
			}
		}
	}

	ate := make(map[int32]bool)
	for playerID, snake := range gs.Snakes {
		newHeadPos := gs.Field.Move(snake.Head(), snake.HeadDirection)
		for _, food := range gs.Foods {
			if food.Equals(newHeadPos) {
				ate[playerID] = true
				break
			}
		}
	}

	for _, snake := range gs.Snakes {
		snake.Move(gs.Field, ate[snake.PlayerID])
	}

	newFoods := make([]Coord, 0, len(gs.Foods))
	for _, food := range gs.Foods {
		eaten := false

		for playerID, snake := range gs.Snakes {
			if snake.Head().Equals(food) {
				eaten = true

				if snake.State == SnakeStateAlive {
					if player, ok := gs.Players[playerID]; ok {
						player.Score++
						result.ScoreChanges[playerID]++
					}
				}
				break
			}
		}
		if !eaten {
			newFoods = append(newFoods, food)
		}
	}
	gs.Foods = newFoods

	killed := make(map[int32]bool)
	killerOf := make(map[int32]int32)

	for playerID, snake := range gs.Snakes {
		headPos := snake.Head()

		for otherID, otherSnake := range gs.Snakes {
			body := otherSnake.Body(gs.Field)
			for i, cell := range body {
				if cell.Equals(headPos) {
					if playerID == otherID && i == 0 {
						continue
					}

					if i == 0 && playerID != otherID {
						killed[playerID] = true
						killed[otherID] = true
					} else {
						killed[playerID] = true
						if playerID != otherID {
							killerOf[playerID] = otherID
						}
					}
				}
			}
		}
	}

	for victimID, killerID := range killerOf {
		if victimID == killerID {
			continue
		}
		if snake, ok := gs.Snakes[killerID]; ok && snake.State == SnakeStateAlive {
			if player, ok := gs.Players[killerID]; ok {
				player.Score++
				result.ScoreChanges[killerID]++
			}
		}

	}

	for playerID := range killed {
		snake := gs.Snakes[playerID]
		body := snake.Body(gs.Field)

		for _, cell := range body {
			if rand.Float32() < 0.5 {
				hasFood := false
				for _, food := range gs.Foods {
					if food.Equals(cell) {
						hasFood = true
						break
					}
				}
				if !hasFood {
					gs.Foods = append(gs.Foods, cell)
				}
			}
		}

		delete(gs.Snakes, playerID)
		result.KilledPlayers = append(result.KilledPlayers, playerID)

		if player, ok := gs.Players[playerID]; ok {
			if player.Role != RoleViewer {
				player.Role = RoleViewer
			}
		}
	}

	gs.ensureFoodUnlocked()

	return result
}

func (gs *GameState) ensureFoodUnlocked() {
	required := int(gs.Config.FoodStatic) + gs.aliveSnakesCountUnlocked()

	occupied := make(map[Coord]bool)
	for _, snake := range gs.Snakes {
		for _, cell := range snake.Body(gs.Field) {
			occupied[cell] = true
		}
	}
	for _, food := range gs.Foods {
		occupied[food] = true
	}

	for len(gs.Foods) < required {
		found := false
		for attempts := 0; attempts < 100; attempts++ {
			x := rand.Int31n(gs.Field.Width)
			y := rand.Int31n(gs.Field.Height)
			pos := Coord{x, y}

			if !occupied[pos] {
				gs.Foods = append(gs.Foods, pos)
				occupied[pos] = true
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
}

func (gs *GameState) aliveSnakesCountUnlocked() int {
	count := 0
	for _, snake := range gs.Snakes {
		if snake.State == SnakeStateAlive {
			count++
		}
	}
	return count
}

func (gs *GameState) MakeSnakeZombie(playerID int32) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if snake, ok := gs.Snakes[playerID]; ok {
		snake.State = SnakeStateZombie
	}
}
