package domain

type GameConfig struct {
	Width        int32
	Height       int32
	FoodStatic   int32
	StateDelayMs int32
}

func DefaultGameConfig() *GameConfig {
	return &GameConfig{
		Width:        40,
		Height:       30,
		FoodStatic:   25,
		StateDelayMs: 200,
	}
}

func (c *GameConfig) Validate() bool {
	if c.Width < 10 || c.Width > 100 {
		return false
	}
	if c.Height < 10 || c.Height > 100 {
		return false
	}
	if c.FoodStatic < 0 || c.FoodStatic > 100 {
		return false
	}
	if c.StateDelayMs < 100 || c.StateDelayMs > 3000 {
		return false
	}
	return true
}

func (c *GameConfig) Copy() *GameConfig {
	return &GameConfig{
		Width:        c.Width,
		Height:       c.Height,
		FoodStatic:   c.FoodStatic,
		StateDelayMs: c.StateDelayMs,
	}
}
