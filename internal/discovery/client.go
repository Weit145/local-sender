package discovery

import (
	"context"
	"net"
)

// FindServers treats its own search deadline as successful completion.
func (d *Discover) FindServers(parent context.Context, localIP net.IP) error {
	ctx, cancel := context.WithTimeout(parent, d.timeout)
	defer cancel()
	d.clearServers()
	if err := ctx.Err(); err != nil {
		return err
	}
	addr, err := net.ResolveUDPAddr("udp4", d.broadcastAddr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: localIP})
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if _, err := conn.WriteToUDP([]byte("DISCOVER_SERVERS"), addr); err != nil {
		return err
	}
	buf := make([]byte, 1024)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if ctx.Err() != nil {
			if parent.Err() != nil {
				return parent.Err()
			}
			return nil
		}
		if err != nil {
			return err
		}
		d.AddServer(remote.IP.String(), string(buf[:n]))
	}
}

func (d *Discover) ReadInterfaces() ([]InterfaceAddress, error) { return ReadInterfaces() }

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
