package controller

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"lab1/internal/data"
	"lab1/internal/messages"
	"lab1/internal/output"
	"lab1/internal/processing"
)

var ifaceName = flag.String("iface", "", "network interface to use (optional)")

type controller struct {
	lib *data.InfoStruct
}

func Controller() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Println("Usage: go run main.go [-iface en0] <multicastAddr:port>")
		return
	}
	group := flag.Arg(0)
	var cnt controller
	infStr, err := data.NewInfoStruct(group, *ifaceName)
	cnt.lib = infStr
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	cnt.run()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	fmt.Println("\n===SHUTDOWN...===")
	cnt.Shutdown()
	fmt.Println("\n===SHUTDOWN END===")
}

func (cnt *controller) run() {
	cnt.lib.Wg.Add(1)
	go func() {
		defer cnt.lib.Wg.Done()
		messages.SendConnect(cnt.lib, cnt.lib.Ctx)
	}()

	cnt.lib.Wg.Add(1)
	go func() {
		defer cnt.lib.Wg.Done()
		messages.Receive(cnt.lib, cnt.lib.Ctx)
	}()

	cnt.lib.Wg.Add(1)
	go func() {
		defer cnt.lib.Wg.Done()
		processing.Cleanup(cnt.lib, cnt.lib.Ctx)
	}()

	cnt.lib.Wg.Add(1)
	go func() {
		defer cnt.lib.Wg.Done()
		output.PrintConnections(cnt.lib, cnt.lib.Ctx)
	}()
}

func (cnt *controller) Shutdown() {
	cnt.lib.Cancel()

	if cnt.lib.RecvConn != nil {
		_ = cnt.lib.RecvConn.Close()
	}
	if cnt.lib.SendConn != nil {
		_ = cnt.lib.SendConn.Close()
	}

	cnt.lib.Wg.Wait()
}
