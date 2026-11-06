package connect

import (
	"bufio"
	"fmt"
	"net"
)

type ServerConnect struct {
}

func NewServerConnect() *ServerConnect {
	return &ServerConnect{}
}

func (s *ServerConnect) Handler(conn net.Conn) {
	defer conn.Close()
	fmt.Printf("Клиент подключился: %s\n", conn.RemoteAddr())
	defer fmt.Printf("Клиент отключился: %s\n", conn.RemoteAddr())

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fmt.Printf("Получено сообщение от %s: %s\n", conn.RemoteAddr(), scanner.Text())
		if _, err := fmt.Fprintln(conn, "Сообщение получено"); err != nil {
			fmt.Println("Ошибка отправки ответа:", err)
			return
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Println("Ошибка чтения из соединения:", err)
	}
}
