package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Weit145/local-sender/cmd/internal/connect"
	"github.com/Weit145/local-sender/cmd/internal/discovery"
)

func main() {
	tcpAddr := flag.String("tcp", ":8080", "TCP-адрес сервера")
	udpAddr := flag.String("udp", ":9999", "UDP-адрес сервера обнаружения")
	broadcastAddr := flag.String("broadcast", "255.255.255.255:9999", "Адрес поиска серверов")
	timeout := flag.Duration("timeout", 3*time.Second, "Время поиска серверов")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	input := bufio.NewScanner(os.Stdin)
	fmt.Println("Выберите режим:\n1. Сервер\n2. Клиент\n0. Выход")
	mode, err := readChoice(input, 2)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch mode {
	case 1:
		err = runServer(ctx, *tcpAddr, *udpAddr)
	case 2:
		err = runClient(ctx, *broadcastAddr, *timeout, input)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServer(ctx context.Context, tcpAddr, udpAddr string) error {
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

		go Handler(conn)
	}
}

func runClient(ctx context.Context, broadcastAddr string, timeout time.Duration, input *bufio.Scanner) error {
	if timeout <= 0 {
		return fmt.Errorf("время поиска должно быть положительным")
	}
	searchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	fmt.Println("Поиск серверов...")
	servers := discovery.NewDiscovery(broadcastAddr).ListenUPD(searchCtx, timeout)
	var addresses []string
	seen := make(map[string]bool)
	for server := range servers {
		port, ok := strings.CutPrefix(server.Data, "MY_TCP_PORT:")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			continue
		}
		addr := net.JoinHostPort(server.IP, port)
		if !seen[addr] {
			seen[addr] = true
			addresses = append(addresses, addr)
			fmt.Printf("%d. %s\n", len(addresses), addr)
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	if len(addresses) == 0 {
		return fmt.Errorf("серверы не найдены за %s", timeout)
	}
	fmt.Println("Выберите номер сервера для подключения (0 — выход).")
	choice, err := readChoice(input, len(addresses))
	if err != nil {
		return err
	}
	if choice != 0 {
		connect.NewConnect().ConnectWithInput(ctx, addresses[choice-1], input)
	}
	return nil
}

func readChoice(input *bufio.Scanner, max int) (int, error) {
	for {
		fmt.Print("> ")
		if !input.Scan() {
			return 0, input.Err()
		}
		choice, err := strconv.Atoi(strings.TrimSpace(input.Text()))
		if err == nil && choice >= 0 && choice <= max {
			return choice, nil
		}
		fmt.Printf("Введите число от 0 до %d.\n", max)
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
