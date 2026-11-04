package discovery

import (
	"context"
	"fmt"
	"net"
)

type ServerListener struct {
	udpAddr string
	tcpPort string
}

func NewServerListener(udpAddr, tcpPort string) *ServerListener {
	return &ServerListener{
		udpAddr: udpAddr,
		tcpPort: tcpPort,
	}
}

func (s *ServerListener) Start(ctx context.Context) {
	addr, err := net.ResolveUDPAddr("udp", s.udpAddr)
	if err != nil {
		fmt.Println("[UDP Сервер] Ошибка адреса:", err)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("[UDP Сервер] Ошибка запуска:", err)
		return
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	fmt.Printf("[UDP Сервер] Слушает порт %s для автообнаружения...\n", s.udpAddr)

	buf := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				fmt.Println("[UDP Сервер] Остановка по сигналу контекста.")
				return
			}
			fmt.Println("[UDP Сервер] Ошибка чтения пакета:", err)
			return
		}

		message := string(buf[:n])

		if message == "DISCOVER_SERVERS" {
			fmt.Printf("[UDP Сервер] Перехвачен запрос от %s\n", remoteAddr.String())

			responseData := fmt.Sprintf("MY_TCP_PORT:%s", s.tcpPort)

			_, err = conn.WriteToUDP([]byte(responseData), remoteAddr)
			if err != nil {
				fmt.Println("[UDP Сервер] Не удалось ответить клиенту:", err)
			}
		}
	}
}
