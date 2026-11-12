package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/Weit145/local-sender/internal/console"
	"github.com/Weit145/local-sender/internal/discovery"
)

type Connect interface {
	ConnectWithInput(ctx context.Context, addr string, input *console.Input)
}

func RunClient(ctx context.Context, broadcastAddr string, timeout time.Duration, input *console.Input, connect Connect) error {
	if timeout <= 0 {
		return fmt.Errorf("время поиска должно быть положительным")
	}

	for ctx.Err() == nil {
		interfaces, err := discovery.ReadInterfaces()
		if err != nil {
			return fmt.Errorf("чтение сетевых интерфейсов: %w", err)
		}
		if len(interfaces) == 0 {
			return fmt.Errorf("нет активных IPv4-интерфейсов для поиска")
		}
		fmt.Println("Выберите сетевой интерфейс для поиска:")
		for i, iface := range interfaces {
			fmt.Printf("%d. %s — %s\n", i+1, iface.Name, iface.Network)
		}
		fmt.Println("0. Выход")
		choice, err := readChoice(ctx, input, len(interfaces))
		if err != nil {
			return err
		}
		if choice == 0 || ctx.Err() != nil {
			return nil
		}
		localIP := interfaces[choice-1].Network.IP.To4()
		fmt.Println("Поиск серверов...")
		servers := discovery.NewDiscovery(broadcastAddr, localIP).ListenUPD(ctx, timeout)
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
			choice, err := readChoice(ctx, input, 1)
			if err != nil {
				return err
			}
			if choice == 0 {
				return nil
			}
			continue
		}
		fmt.Println("Выберите номер сервера для подключения (0 — выход).")
		choice, err = readChoice(ctx, input, len(addresses))
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
