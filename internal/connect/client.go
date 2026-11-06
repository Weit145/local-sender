package connect

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

type ClientConnect struct {
}

func NewClientConnect() *ClientConnect {
	return &ClientConnect{}
}

func (c *ClientConnect) Connect(ctx context.Context, addr string) {
	c.ConnectWithInput(ctx, addr, bufio.NewScanner(os.Stdin))
}

func (c *ClientConnect) ConnectWithInput(ctx context.Context, addr string, scanner *bufio.Scanner) {
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

		fmt.Println("Подключено к серверу. Введите сообщение и нажмите Enter (для выхода введите 'q'):")
		reader := bufio.NewReader(conn)

		for scanner.Scan() {
			text := scanner.Text()
			if strings.ToLower(text) == "q" {
				return
			}

			_, err := fmt.Fprintf(conn, "%s\n", text)
			if err != nil {
				fmt.Println("Ошибка отправки:", err)
				return
			}

			serverReply, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Ошибка чтения ответа:", err)
				return
			}
			fmt.Print("Ответ сервера: ", serverReply)
		}
	}

}
