package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

type ServerInfo struct {
	IP   string
	Data string
}

type Discovery struct {
	addr string
}

func NewDiscovery(addr string) *Discovery {
	return &Discovery{addr: addr}
}

func (d *Discovery) ListenUPD(ctx context.Context, timeout time.Duration) <-chan ServerInfo {
	out := make(chan ServerInfo, 100)
	go d.readServer(ctx, out, timeout)
	return out
}

type discoveryTarget struct {
	local  *net.UDPAddr
	remote *net.UDPAddr
}

// subnetBroadcast returns the broadcast address for an IPv4 subnet.
// Point-to-point networks (/31 and /32) have no broadcast address.
func subnetBroadcast(network *net.IPNet) net.IP {
	ip := network.IP.To4()
	ones, bits := network.Mask.Size()
	if ip == nil || bits != 32 || ones > 30 {
		return nil
	}
	broadcast := make(net.IP, net.IPv4len)
	for i := range broadcast {
		broadcast[i] = ip[i] | ^network.Mask[i]
	}
	return broadcast
}

func discoveryTargets(addr *net.UDPAddr) ([]discoveryTarget, error) {
	// An explicitly supplied address also allows a direct query to a server.
	if !addr.IP.Equal(net.IPv4bcast) {
		return []discoveryTarget{{local: &net.UDPAddr{}, remote: addr}}, nil
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var targets []discoveryTarget
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagBroadcast == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			fmt.Printf("[UDP Поиск] Интерфейс %s: %v\n", iface.Name, err)
			continue
		}
		for _, address := range addresses {
			network, ok := address.(*net.IPNet)
			if !ok {
				continue
			}
			broadcast := subnetBroadcast(network)
			if broadcast == nil {
				continue
			}
			local := &net.UDPAddr{IP: network.IP.To4()}
			targets = append(targets, discoveryTarget{local: local, remote: &net.UDPAddr{IP: broadcast, Port: addr.Port}})
			// Also try the limited broadcast address through this interface.
			if !broadcast.Equal(addr.IP) {
				targets = append(targets, discoveryTarget{local: local, remote: addr})
			}
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("нет активных IPv4-интерфейсов для обнаружения")
	}
	return targets, nil
}

func (d *Discovery) readServer(ctx context.Context, out chan<- ServerInfo, timeout time.Duration) {
	defer close(out)
	addr, err := net.ResolveUDPAddr("udp4", d.addr)
	if err != nil {
		fmt.Println("[UDP Поиск] Ошибка адреса:", err)
		return
	}
	targets, err := discoveryTargets(addr)
	if err != nil {
		fmt.Println("[UDP Поиск] Ошибка интерфейсов:", err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var workers sync.WaitGroup
	for _, target := range targets {
		workers.Add(1)
		go func() {
			defer workers.Done()
			queryServer(ctx, target, out)
		}()
	}
	workers.Wait()
}

func queryServer(ctx context.Context, target discoveryTarget, out chan<- ServerInfo) {
	conn, err := net.ListenUDP("udp4", target.local)
	if err != nil {
		fmt.Println("[UDP Поиск] Ошибка открытия сокета:", err)
		return
	}
	defer conn.Close()
	fmt.Printf("[UDP Поиск] %s → %s\n", conn.LocalAddr(), target.remote)
	buf := make([]byte, 1024)
	if ctx.Err() != nil {
		return
	}
	if _, err := conn.WriteToUDP([]byte("DISCOVER_SERVERS"), target.remote); err != nil {
		fmt.Printf("[UDP Поиск] Ошибка отправки на %s: %v\n", target.remote, err)
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
