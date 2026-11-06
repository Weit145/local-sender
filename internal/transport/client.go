package transport

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Weit145/local-sender/internal/discovery"
)

type Connect interface {
	ConnectWithInput(ctx context.Context, addr string, scanner *bufio.Scanner)
}

func RunClient(ctx context.Context, broadcastAddr string, timeout time.Duration, input *bufio.Scanner, connect Connect) error {
	if timeout <= 0 {
		return fmt.Errorf("время поиска должно быть положительным")
	}

	for ctx.Err() == nil {
		fmt.Println("Поиск серверов...")
		servers := discovery.NewDiscovery(broadcastAddr).ListenUPD(ctx, timeout)
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
			fmt.Printf("Серверы не найдены за %s.\n", timeout)
			fmt.Println("1. Повторить поиск\n0. Выход")
			choice, err := readChoice(input, 1)
			if err != nil {
				return err
			}
			if choice == 0 {
				return nil
			}
			continue
		}
		fmt.Println("Выберите номер сервера для подключения (0 — выход).")
		choice, err := readChoice(input, len(addresses))
		if err != nil {
			return err
		}
		if choice != 0 {
			connect.ConnectWithInput(ctx, addresses[choice-1], input)
		}
		return nil
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
