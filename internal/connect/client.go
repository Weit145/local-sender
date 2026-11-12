package connect

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Weit145/local-sender/internal/console"
)

type ClientConnect struct {
}

func NewClientConnect() *ClientConnect {
	return &ClientConnect{}
}

func (c *ClientConnect) ConnectWithInput(ctx context.Context, addr string, input *console.Input) {
	select {
	case <-ctx.Done():
		return
	default:
		fmt.Printf("Подключение к %s (таймаут 5 секунд)...\n", addr)
		dialer := net.Dialer{Timeout: 5 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			fmt.Println("Не удалось подключиться к серверу:", err)
			return
		}
		defer conn.Close()

		ctxDone := make(chan struct{})
		defer close(ctxDone)

		go func() {
			select {
			case <-ctx.Done():
				conn.Close()
			case <-ctxDone:
				return
			}
		}()

		reader := bufio.NewReader(conn)
		fmt.Println("Ожидание разрешения от сервера...")
		approval, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Не удалось получить разрешение сервера:", err)
			return
		}
		switch strings.TrimSpace(approval) {
		case "ACCEPTED":
			fmt.Println("Сервер разрешил подключение. Введите сообщение и нажмите Enter (для выхода введите 'q'):")
		case "REJECTED":
			fmt.Println("Сервер отклонил подключение.")
			return
		default:
			fmt.Printf("Неизвестный ответ на запрос подключения: %q\n", approval)
			return
		}
		ConnectTCP(ctx, conn, reader, input)
	}

}
