package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Weit145/local-sender/internal/connect"
	"github.com/Weit145/local-sender/internal/console"
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

	input := console.NewInput(ctx, os.Stdin)
	fmt.Println("Выберите режим:\n1. Сервер\n2. Клиент\n0. Выход")
	mode, err := readChoice(ctx, input, 2)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch mode {
	case 1:
		err = transport.RunServer(ctx, *tcpAddr, *udpAddr, connect.NewServerConnect(), input)
	case 2:
		err = transport.RunClient(ctx, *broadcastAddr, *timeout, input, con)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readChoice(ctx context.Context, input *console.Input, max int) (int, error) {
	for {
		fmt.Print("> ")
		text, err := input.Read(ctx)
		if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
			return 0, nil
		}
		if err != nil {
			return 0, err
		}
		choice, err := strconv.Atoi(strings.TrimSpace(text))
		if err == nil && choice >= 0 && choice <= max {
			return choice, nil
		}
		fmt.Printf("Введите число от 0 до %d.\n", max)
	}
}
