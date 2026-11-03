package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	listner, err := net.Listen("tcp", ":8080")
	if err != nil {
		os.Exit(1)
	}

	defer listner.Close()

	for {
		conn, err := listner.Accept()
		if err != nil {
			continue
		}

		go Handler(conn)

	}
}

func Handler(conn net.Conn) {
	defer conn.Close()

	fmt.Printf("Клиент подключился: %s\n", conn.RemoteAddr().String())

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		text := scanner.Text()
		fmt.Printf("Получено сообщение от %s: %s\n", conn.RemoteAddr(), text)

		conn.Write([]byte("Сообщение получено\n"))
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Ошибка чтения из соединения:", err)
	}
	fmt.Printf("Клиент отключился: %s\n", conn.RemoteAddr())
}
