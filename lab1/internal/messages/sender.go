package messages

import (
	"context"
	"fmt"
	"time"

	"lab1/internal/data"
)

func SendConnect(lib *data.InfoStruct, ctx context.Context) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			msg := []byte("CONNECT:" + lib.Id)
			if lib.SendConn == nil {
				continue
			}
			if _, err := lib.SendConn.Write(msg); err != nil {
				select {
				case <-ctx.Done():
					return
				default:
				}
				fmt.Println("send error:", err)
			}
		}
	}
}
