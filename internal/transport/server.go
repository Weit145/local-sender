package transport

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/Weit145/local-sender/internal/discovery"
)

type ServerConnection interface {
	Handler(conn net.Conn)
}

func RunServer(ctx context.Context, tcpAddr, udpAddr string, connection ServerConnection) error {
	listener, err := net.Listen("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("запуск TCP-сервера: %w", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	go discovery.NewServerListener(udpAddr, port).Start(ctx)
	fmt.Printf("TCP-сервер слушает %s\n", listener.Addr())
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("приём TCP-подключения: %w", err)
		}

		go connection.Handler(conn)
	}
}
