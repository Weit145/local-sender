package discovery

import (
	"maps"
	"net"
	"sync"
	"time"
)

type ServerInfo struct {
	IP   string
	Data string
}

type Servers struct {
	all map[string]string
	mx  sync.Mutex
}

type Discover struct {
	tcpAddr       string
	udpAddr       string
	broadcastAddr string
	localIP       net.IP
	timeout       time.Duration
	servers       Servers
}

func NewDiscover(
	tcpAddr,
	udpAddr,
	broadcastAddr string,
	timeout time.Duration,
) *Discover {
	return &Discover{
		tcpAddr:       tcpAddr,
		udpAddr:       udpAddr,
		broadcastAddr: broadcastAddr,
		timeout:       timeout,
		servers:       Servers{all: make(map[string]string)},
	}
}

func (d *Discover) GetServers() map[string]string {
	d.servers.mx.Lock()
	defer d.servers.mx.Unlock()
	return maps.Clone(d.servers.all)
}

func (d *Discover) AddServer(ip, data string) {
	d.servers.mx.Lock()
	defer d.servers.mx.Unlock()
	d.servers.all[ip] = string(data)
}

func (d *Discover) clearServers() {
	d.servers.mx.Lock()
	defer d.servers.mx.Unlock()
	clear(d.servers.all)
}
