package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

type ServerInfo struct {
	IP   string
	Data string
}

type Discovery struct {
	addr    string
	localIP net.IP
}

func NewDiscovery(addr string, localIP net.IP) *Discovery {
	return &Discovery{addr: addr, localIP: localIP}
}

func (d *Discovery) ListenUPD(ctx context.Context, timeout time.Duration) <-chan ServerInfo {
	out := make(chan ServerInfo, 100)
	go d.readServer(ctx, out, timeout)
	return out
}

func (d *Discovery) readServer(ctx context.Context, out chan<- ServerInfo, timeout time.Duration) {
	defer close(out)
	addr, err := net.ResolveUDPAddr("udp4", d.addr)
	if err != nil {
		fmt.Println("[UDP Поиск] Ошибка адреса:", err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if ctx.Err() != nil {
		return
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: d.localIP})
	if err != nil {
		fmt.Println("[UDP Поиск] Ошибка открытия сокета:", err)
		return
	}
	defer conn.Close()
	fmt.Printf("[UDP Поиск] %s → %s\n", conn.LocalAddr(), addr)
	buf := make([]byte, 1024)
	if ctx.Err() != nil {
		return
	}
	if _, err := conn.WriteToUDP([]byte("DISCOVER_SERVERS"), addr); err != nil {
		fmt.Printf("[UDP Поиск] Ошибка отправки на %s: %v\n", addr, err)
		return
	}
	for ctx.Err() == nil {
		if err := conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
			fmt.Println("[UDP Поиск] Ошибка установки таймаута:", err)
			return
		}
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			fmt.Println("[UDP Поиск] Ошибка чтения:", err)
			return
		}
		select {
		case out <- ServerInfo{IP: remote.IP.String(), Data: string(buf[:n])}:
		case <-ctx.Done():
			return
		}
	}
}

type InterfaceAddress struct {
	Name    string
	Network *net.IPNet
}

func ReadInterfaces() ([]InterfaceAddress, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var result []InterfaceAddress

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, address := range addresses {
			network, ok := address.(*net.IPNet)
			if !ok || network.IP.To4() == nil {
				continue
			}
			result = append(result, InterfaceAddress{Name: iface.Name, Network: network})
		}
	}
	return result, nil
}
