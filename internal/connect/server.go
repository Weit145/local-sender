package connect

import (
	"bufio"
	"context"
	"fmt"
	"net"

	"github.com/Weit145/local-sender/internal/console"
)

type ServerConnect struct{}

func NewServerConnect() *ServerConnect { return &ServerConnect{} }

func (s *ServerConnect) Handler(ctx context.Context, conn net.Conn, input *console.Input) {
	fmt.Printf("Клиент подключился: %s. Введите сообщение (q — завершить чат).\n", conn.RemoteAddr())
	defer fmt.Printf("Клиент отключился: %s\n", conn.RemoteAddr())
	ConnectTCP(ctx, conn, bufio.NewReader(conn), input)
}
