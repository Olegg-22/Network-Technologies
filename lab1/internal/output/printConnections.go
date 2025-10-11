package output

import (
	"context"
	"fmt"

	"lab1/internal/data"
)

func PrintConnections(lib *data.InfoStruct, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-lib.PrintCh:
			lib.Mu.Lock()
			id := lib.Id
			selfAddr := lib.SelfAddr
			conns := make([]data.Connection, 0, len(lib.Connections))
			for _, c := range lib.Connections {
				conns = append(conns, c)
			}
			lib.Mu.Unlock()

			fmt.Println("===================================")
			fmt.Printf("🟢 Myself: %s (%s)\n", id, selfAddr)
			fmt.Println("Connections:")
			if len(conns) == 0 {
				fmt.Println("  (none)")
			} else {
				for _, p := range conns {
					fmt.Printf("  🔸 %s (%s)\n", p.ID, p.Addr)
				}
			}
			fmt.Println("===================================")
		}
	}
}
