package connect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/Weit145/local-sender/internal/console"
)

func ConnectTCP(ctx context.Context, conn net.Conn, reader *bufio.Reader, input *console.Input) {
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer conn.Close()
	done := make(chan struct{}, 2)
	go func() {
		defer func() { done <- struct{}{} }()
		readTCP(sessionCtx, conn, reader)
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		writeTCP(sessionCtx, conn, input)
	}()

	completed := 0
	select {
	case <-ctx.Done():
	case <-done:
		completed = 1
	}
	cancel()     // Interrupt waiting for console input.
	conn.Close() // Interrupt network reads and writes.
	for ; completed < 2; completed++ {
		<-done
	}
}

func readTCP(ctx context.Context, conn net.Conn, reader *bufio.Reader) {
	for {
		text, err := reader.ReadString('\n')
		if ctx.Err() != nil {
			return
		}
		if len(text) > 0 {
			fmt.Printf("Получено сообщение от %s: %s\n", conn.RemoteAddr(), strings.TrimSuffix(text, "\n"))
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				fmt.Println("Ошибка чтения из соединения:", err)
			}
			return
		}
	}
}

func writeTCP(ctx context.Context, conn net.Conn, input *console.Input) {
	for {
		text, err := input.Read(ctx)
		if err != nil {
			if ctx.Err() == nil && !errors.Is(err, io.EOF) {
				fmt.Println("Ошибка ввода:", err)
			}
			return
		}
		if strings.EqualFold(text, "q") {
			return
		}
		if ctx.Err() != nil {
			return
		}
		if _, err := fmt.Fprintln(conn, text); err != nil {
			if ctx.Err() == nil {
				fmt.Println("Ошибка отправки:", err)
			}
			return
		}
	}
}
