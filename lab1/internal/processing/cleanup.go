package processing

import (
	"context"
	"time"

	"lab1/internal/data"
)

func Cleanup(lib *data.InfoStruct, ctx context.Context) {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now()
			changed := false
			lib.Mu.Lock()
			for id, p := range lib.Connections {
				if now.Sub(p.LastSeen) > 6*time.Second {
					delete(lib.Connections, id)
					changed = true
				}
			}
			lib.Mu.Unlock()

			if changed {
				select {
				case lib.PrintCh <- struct{}{}:
				default:
				}
			}
		}
	}
}
