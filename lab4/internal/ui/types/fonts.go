package types

import (
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

type Fonts struct {
	Normal font.Face
	Small  font.Face
}

var defaultFonts *Fonts

func InitFonts() {
	defaultFonts = &Fonts{
		Normal: basicfont.Face7x13,
		Small:  basicfont.Face7x13,
	}
}

func GetFonts() *Fonts {
	if defaultFonts == nil {
		InitFonts()
	}
	return defaultFonts
}
