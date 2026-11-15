package discovery

import (
	"context"
	"fmt"
	"net"
)

func (d *Discover) FindClient(ctx context.Context, ready chan<- error) error {
	addr, err := net.ResolveUDPAddr("udp4", d.udpAddr)
	if err != nil {
		ready <- err
		return err
	}
	conn, err := net.ListenUDP("udp4", addr)
	ready <- err
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	buf := make([]byte, 1024)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		if string(buf[:n]) == "DISCOVER_SERVERS" {
			if _, err := conn.WriteToUDP([]byte(fmt.Sprintf("MY_TCP_PORT:%s", d.tcpAddr)), remote); err != nil {
				return err
			}
		}
	}
}
