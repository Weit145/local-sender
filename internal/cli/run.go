// Package cli provides a small example of consuming the UI-facing usecase API.
package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Weit145/local-sender/internal/connect"
	"github.com/Weit145/local-sender/internal/discovery"
	"github.com/Weit145/local-sender/internal/usecase"
)

func Run() error {
	tcp := flag.String("tcp", ":8080", "TCP listen address")
	udp := flag.String("udp", ":9999", "UDP discovery listen address")
	broadcast := flag.String("broadcast", "255.255.255.255:9999", "Discovery destination")
	timeout := flag.Duration("timeout", 3*time.Second, "Discovery duration")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	service := usecase.NewUsecase(connect.NewConnect(*tcp), discovery.NewDiscover(*tcp, *udp, *broadcast, *timeout))
	if err := service.Start(ctx); err != nil {
		return err
	}
	defer service.Stop()
	eventsCtx, stopEvents := context.WithCancel(ctx)
	defer stopEvents()
	go func() {
		for {
			select {
			case <-eventsCtx.Done():
				return
			case event := <-service.Events():
				fmt.Printf("event: %+v\n", event)
			}
		}
	}()
	fmt.Println("Commands: interfaces, discover <local-ip>, devices, connections, pending, connect <host:port>, accept <id>, reject <id>, disconnect <id>, send <id> <text>, quit")
	lines := make(chan string)
	scanErrors := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-eventsCtx.Done():
				return
			}
		}
		scanErrors <- scanner.Err()
		close(lines)
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case line, ok := <-lines:
			if !ok {
				return <-scanErrors
			}
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			var err error
			switch {
			case fields[0] == "quit":
				return nil
			case fields[0] == "interfaces":
				var result []usecase.InterfaceAddress
				result, err = service.ListInterfaces()
				fmt.Printf("%+v\n", result)
			case fields[0] == "devices":
				fmt.Printf("%+v\n", service.ListDevices())
			case fields[0] == "connections":
				fmt.Printf("%+v\n", service.ListConnections())
			case fields[0] == "pending":
				fmt.Printf("%+v\n", service.ListPendingRequests())
			case fields[0] == "discover" && len(fields) == 2:
				_, err = service.DiscoverDevices(ctx, fields[1])
			case fields[0] == "connect" && len(fields) == 2:
				_, err = service.Connect(ctx, fields[1])
			case fields[0] == "accept" && len(fields) == 2:
				err = service.AcceptConnection(fields[1])
			case fields[0] == "reject" && len(fields) == 2:
				err = service.RejectConnection(fields[1])
			case fields[0] == "disconnect" && len(fields) == 2:
				err = service.Disconnect(fields[1])
			case fields[0] == "send" && len(fields) >= 3:
				err = service.SendMessage(ctx, fields[1], strings.Join(fields[2:], " "))
			default:
				err = fmt.Errorf("unknown command or missing arguments")
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
}
