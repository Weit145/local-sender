package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	addr := "127.0.0.1:8080"

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println("Не удалось подключиться к серверу:", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Println("Подключено к серверу. Введите сообщение и нажмите Enter (для выхода введите 'q'):")
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		text := scanner.Text()
		if strings.ToLower(text) == "q" {
			break
		}

		// Отправляем сообщение на сервер с переходом строки
		_, err := fmt.Fprintf(conn, "%s\n", text)
		if err != nil {
			fmt.Println("Ошибка отправки:", err)
			break
		}

		// Читаем ответ от сервера
		serverReply, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			fmt.Println("Ошибка чтения ответа:", err)
			break
		}
		fmt.Print("Ответ сервера: ", serverReply)
	}
}
