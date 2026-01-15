package components

import (
	"fmt"
	"sort"

	"lab4/internal/domain"
	"lab4/internal/ui/types"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Scoreboard struct {
	X, Y          int
	Width, Height int
}

func NewScoreboard(x, y, width, height int) *Scoreboard {
	return &Scoreboard{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
	}
}

func (sb *Scoreboard) Draw(screen *ebiten.Image, players map[int32]*domain.Player, myID int32) {
	vector.DrawFilledRect(screen,
		float32(sb.X), float32(sb.Y),
		float32(sb.Width), float32(sb.Height),
		types.Darken(types.ColorFieldBg, 0.8), false)

	vector.StrokeRect(screen,
		float32(sb.X), float32(sb.Y),
		float32(sb.Width), float32(sb.Height),
		1, types.ColorGrid, false)

	fonts := types.GetFonts()

	text.Draw(screen, "PLAYERS", fonts.Normal, sb.X+10, sb.Y+20, types.ColorTextHighlight)

	type playerScore struct {
		player *domain.Player
		id     int32
	}
	var sorted []playerScore
	for id, p := range players {
		if p.Role != domain.RoleViewer {
			sorted = append(sorted, playerScore{p, id})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].player.Score != sorted[j].player.Score {
			return sorted[i].player.Score > sorted[j].player.Score
		}
		return sorted[i].id < sorted[j].id
	})

	y := sb.Y + 45
	for i, ps := range sorted {
		if y > sb.Y+sb.Height-20 {
			break
		}

		snakeColor := types.GetSnakeColor(ps.id)
		vector.DrawFilledRect(screen,
			float32(sb.X+10), float32(y-10),
			12, 12,
			snakeColor, false)

		name := ps.player.Name
		if len(name) > 12 {
			name = name[:12] + "..."
		}

		textColor := types.ColorText
		if ps.id == myID {
			textColor = types.ColorTextHighlight
			name = "► " + name
		}

		line := fmt.Sprintf("%d. %s: %d", i+1, name, ps.player.Score)
		text.Draw(screen, line, fonts.Normal, sb.X+28, y, textColor)

		roleText := ""
		switch ps.player.Role {
		case domain.RoleMaster:
			roleText = "[M]"
		case domain.RoleDeputy:
			roleText = "[D]"
		}
		if roleText != "" {
			text.Draw(screen, roleText, fonts.Small, sb.X+sb.Width-35, y, types.ColorTextDim)
		}

		y += 22
	}
}
