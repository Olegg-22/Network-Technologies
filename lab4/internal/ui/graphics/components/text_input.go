package components

import (
	"strings"
	"unicode/utf8"

	"lab4/internal/ui/types"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type TextInput struct {
	X, Y          int
	Width, Height int
	Text          string
	Placeholder   string
	MaxLength     int
	Focused       bool
	cursorBlink   int
}

func NewTextInput(x, y, width, height int, placeholder string) *TextInput {
	return &TextInput{
		X:           x,
		Y:           y,
		Width:       width,
		Height:      height,
		Placeholder: placeholder,
		MaxLength:   50,
	}
}

func (ti *TextInput) Update() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		ti.Focused = mx >= ti.X && mx < ti.X+ti.Width && my >= ti.Y && my < ti.Y+ti.Height
	}

	if !ti.Focused {
		return
	}

	ti.cursorBlink++

	var runes []rune
	runes = ebiten.AppendInputChars(runes)
	for _, r := range runes {
		if utf8.RuneCountInString(ti.Text) < ti.MaxLength {
			ti.Text += string(r)
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		if len(ti.Text) > 0 {
			ti.Text = ti.Text[:len(ti.Text)-1]
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		ti.Text = strings.TrimRight(ti.Text, " ")
		idx := strings.LastIndex(ti.Text, " ")
		if idx >= 0 {
			ti.Text = ti.Text[:idx+1]
		} else {
			ti.Text = ""
		}
	}
}

func (ti *TextInput) Draw(screen *ebiten.Image) {
	vector.DrawFilledRect(screen,
		float32(ti.X), float32(ti.Y),
		float32(ti.Width), float32(ti.Height),
		types.ColorInputBg, false)

	borderColor := types.ColorInputBorder
	if ti.Focused {
		borderColor = types.ColorInputFocused
	}
	vector.StrokeRect(screen,
		float32(ti.X), float32(ti.Y),
		float32(ti.Width), float32(ti.Height),
		2, borderColor, false)

	fonts := types.GetFonts()
	displayText := ti.Text
	textColor := types.ColorText

	if displayText == "" && !ti.Focused {
		displayText = ti.Placeholder
		textColor = types.ColorTextDim
	}

	textX := ti.X + 8
	textY := ti.Y + ti.Height/2 + 4
	text.Draw(screen, displayText, fonts.Normal, textX, textY, textColor)

	if ti.Focused && (ti.cursorBlink/30)%2 == 0 {
		bounds := text.BoundString(fonts.Normal, ti.Text)
		cursorX := float32(textX + bounds.Dx() + 2)
		cursorY := float32(ti.Y + 5)
		vector.StrokeLine(screen, cursorX, cursorY, cursorX, float32(ti.Y+ti.Height-5), 2, types.ColorText, false)
	}
}

func (ti *TextInput) SetPosition(x, y int) {
	ti.X = x
	ti.Y = y
}

func (ti *TextInput) Clear() {
	ti.Text = ""
}
