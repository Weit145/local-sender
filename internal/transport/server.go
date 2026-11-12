package transport

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/Weit145/local-sender/internal/console"
	"github.com/Weit145/local-sender/internal/discovery"
)

type ServerConnection interface {
	Handler(ctx context.Context, conn net.Conn, input *console.Input)
}

func RunServer(ctx context.Context, tcpAddr, udpAddr string, connection ServerConnection, input *console.Input) error {
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
	for {
		fmt.Printf("TCP-сервер слушает %s\n", listener.Addr())
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("приём TCP-подключения: %w", err)
		}
		fmt.Printf("Клиент %s хочет подключиться.\n1. Разрешить\n2. Отклонить\n", conn.RemoteAddr())
		choice, err := readChoice(ctx, input, 2)
		if err != nil {
			conn.Close()
			return err
		}
		if choice != 1 {
			if _, err := fmt.Fprintln(conn, "REJECTED"); err != nil {
				fmt.Println("Ошибка отправки отказа:", err)
			}
			conn.Close()
			continue
		}
		if _, err := fmt.Fprintln(conn, "ACCEPTED"); err != nil {
			fmt.Println("Ошибка отправки разрешения:", err)
			conn.Close()
			continue
		}
		connection.Handler(ctx, conn, input)

	}
}
