package messages

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"lab1/internal/data"
)

func Receive(lib *data.InfoStruct, ctx context.Context) {
	buf := make([]byte, data.SizeBuffer)
	for {
		_ = lib.RecvConn.SetReadDeadline(time.Now().Add(3 * time.Second))
		n, src, err := lib.RecvConn.ReadFromUDP(buf)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
			fmt.Println("read error:", err)
			continue
		}
		msg := string(buf[:n])
		parts := strings.Split(msg, ":")
		if len(parts) != data.CountReceivedArgument || parts[0] != "CONNECT" {
			continue
		}
		connectionID := parts[1]
		if connectionID == lib.Id {
			continue
		}
		lib.Mu.Lock()
		lib.Connections[connectionID] = data.Connection{ID: connectionID, Addr: src.IP.String(), LastSeen: time.Now()}
		lib.Mu.Unlock()

		select {
		case lib.PrintCh <- struct{}{}:
		default:
		}
	}
}
