package types

import "image/color"

var (
	ColorBackground    = color.RGBA{30, 30, 30, 255}
	ColorFieldBg       = color.RGBA{40, 40, 45, 255}
	ColorGrid          = color.RGBA{60, 60, 65, 255}
	ColorFood          = color.RGBA{255, 80, 80, 255}
	ColorText          = color.RGBA{220, 220, 220, 255}
	ColorTextDim       = color.RGBA{150, 150, 150, 255}
	ColorTextHighlight = color.RGBA{255, 255, 100, 255}
	ColorButton        = color.RGBA{70, 70, 80, 255}
	ColorButtonHover   = color.RGBA{90, 90, 100, 255}
	ColorButtonText    = color.RGBA{220, 220, 220, 255}
	ColorInputBg       = color.RGBA{50, 50, 55, 255}
	ColorInputBorder   = color.RGBA{100, 100, 110, 255}
	ColorInputFocused  = color.RGBA{100, 150, 200, 255}
	ColorError         = color.RGBA{255, 100, 100, 255}
	ColorSuccess       = color.RGBA{100, 255, 100, 255}
)

var SnakeColors = []color.RGBA{
	{100, 200, 100, 255},
	{100, 150, 255, 255},
	{255, 200, 100, 255},
	{200, 100, 255, 255},
	{255, 100, 200, 255},
	{100, 255, 255, 255},
	{255, 255, 100, 255},
	{200, 200, 200, 255},
}

func GetSnakeColor(playerID int32) color.RGBA {
	idx := int(playerID-1) % len(SnakeColors)
	if idx < 0 {
		idx = 0
	}
	return SnakeColors[idx]
}

func Darken(c color.RGBA, factor float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(c.R) * factor),
		G: uint8(float64(c.G) * factor),
		B: uint8(float64(c.B) * factor),
		A: c.A,
	}
}

func Lighten(c color.RGBA, factor float64) color.RGBA {
	return color.RGBA{
		R: uint8(min(255, float64(c.R)*factor)),
		G: uint8(min(255, float64(c.G)*factor)),
		B: uint8(min(255, float64(c.B)*factor)),
		A: c.A,
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
