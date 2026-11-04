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

func (d *Discovery) readServer(ctx context.Context, out chan<- ServerInfo, timeout time.Duration) {
	defer close(out)

	broadcastAddr, err := net.ResolveUDPAddr("udp", d.addr)
	if err != nil {
		fmt.Println("Ошибка ResolveUDPAddr:", err)
		return
	}

	localAddr, _ := net.ResolveUDPAddr("udp", ":0")
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		fmt.Println("Ошибка ListenUDP:", err)
		return
	}
	defer conn.Close()

	_, err = conn.WriteToUDP([]byte("DISCOVER_SERVERS"), broadcastAddr)
	if err != nil {
		fmt.Println("Ошибка отправки пакета:", err)
		return
	}

	buf := make([]byte, 1024)

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			return
		default:
			readTimeout := 200 * time.Millisecond
			_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				return
			}
			select {
			case out <- ServerInfo{
				IP:   remoteAddr.IP.String(),
				Data: string(buf[:n]),
			}:
			case <-ctx.Done():
				return
			}
		}

	}
}
