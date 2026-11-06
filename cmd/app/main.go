package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Weit145/local-sender/internal/connect"
	"github.com/Weit145/local-sender/internal/transport"
)

func main() {
	tcpAddr := flag.String("tcp", ":8080", "TCP-адрес сервера")
	udpAddr := flag.String("udp", ":9999", "UDP-адрес сервера обнаружения")
	broadcastAddr := flag.String("broadcast", "255.255.255.255:9999", "Адрес поиска серверов")
	timeout := flag.Duration("timeout", 3*time.Second, "Время поиска серверов")
	flag.Parse()

	con := connect.NewClientConnect()

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
		err = transport.RunServer(ctx, *tcpAddr, *udpAddr, connect.NewServerConnect())
	case 2:
		err = transport.RunClient(ctx, *broadcastAddr, *timeout, input, con)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
