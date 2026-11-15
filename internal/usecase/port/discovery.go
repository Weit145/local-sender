package port

import (
	"context"
	"github.com/Weit145/local-sender/internal/discovery"
	"net"
)

type Discover interface {
	GetServers() map[string]string
	FindServers(context.Context, net.IP) error
	ReadInterfaces() ([]discovery.InterfaceAddress, error)
	FindClient(context.Context, chan<- error) error
}
