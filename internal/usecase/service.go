package usecase

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Weit145/local-sender/internal/connect"
)

func (u *Usecase) Start(ctx context.Context) error {
	u.lifecycle.Lock()
	defer u.lifecycle.Unlock()
	u.mu.Lock()
	if u.current != nil {
		err := u.current.ctx.Err()
		u.mu.Unlock()
		return err
	}
	if err := ctx.Err(); err != nil {
		u.mu.Unlock()
		return err
	}
	runCtx, fail := context.WithCancelCause(ctx)
	cancel := func() { fail(context.Canceled) }
	r := &run{ctx: runCtx, cancel: cancel, fail: fail}
	u.current = r
	u.mu.Unlock()
	incoming := make(chan *connect.Connection)
	tcpReady, udpReady := make(chan error, 1), make(chan error, 1)
	r.wg.Add(3)
	go func() { defer r.wg.Done(); u.backgroundResult(r, u.con.ListenServer(r.ctx, incoming, tcpReady)) }()
	go func() { defer r.wg.Done(); u.backgroundResult(r, u.disc.FindClient(r.ctx, udpReady)) }()
	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-r.ctx.Done():
				return
			case conn := <-incoming:
				u.incoming(r, conn)
			}
		}
	}()
	for _, ready := range []<-chan error{tcpReady, udpReady} {
		var err error
		select {
		case err = <-ready:
		case <-r.ctx.Done():
			err = context.Cause(r.ctx)
		}
		if err != nil {
			cancel()
			r.wg.Wait()
			u.mu.Lock()
			u.current = nil
			u.mu.Unlock()
			return err
		}
	}
	return context.Cause(r.ctx)
}

func (u *Usecase) backgroundResult(r *run, err error) {
	if err != nil && r.ctx.Err() == nil {
		r.fail(err)
		select {
		case u.events <- Event{Type: EventError, Error: err.Error()}:
		default:
		}
	}
}

func (u *Usecase) Stop() {
	u.lifecycle.Lock()
	defer u.lifecycle.Unlock()
	u.mu.Lock()
	r := u.current
	if r == nil {
		u.mu.Unlock()
		return
	}
	r.cancel()
	u.mu.Unlock()
	r.wg.Wait()
	u.mu.Lock()
	u.current = nil
	u.mu.Unlock()
}

func (u *Usecase) ListInterfaces() ([]InterfaceAddress, error) {
	interfaces, err := u.disc.ReadInterfaces()
	if err != nil {
		return nil, err
	}
	result := make([]InterfaceAddress, 0, len(interfaces))
	for _, item := range interfaces {
		result = append(result, InterfaceAddress{Name: item.Name, IP: item.Network.IP.String(), Network: item.Network.String()})
	}
	return result, nil
}

func (u *Usecase) DiscoverDevices(ctx context.Context, localIP string) ([]Device, error) {
	ip := net.ParseIP(localIP)
	if ip == nil || ip.To4() == nil {
		return nil, fmt.Errorf("invalid IPv4 address: %q", localIP)
	}
	u.mu.Lock()
	r := u.current
	if r == nil || r.ctx.Err() != nil {
		u.mu.Unlock()
		return nil, ErrNotRunning
	}
	if u.searching {
		u.mu.Unlock()
		return nil, ErrSearchInProgress
	}
	u.searching = true
	r.wg.Add(1)
	u.mu.Unlock()
	defer r.wg.Done()
	defer func() { u.mu.Lock(); u.searching = false; u.mu.Unlock() }()
	searchCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(r.ctx, cancel)
	defer stop()
	defer cancel()
	if err := u.disc.FindServers(searchCtx, ip); err != nil {
		return nil, err
	}
	devices := make([]Device, 0)
	for ip, data := range u.disc.GetServers() {
		if net.ParseIP(ip) == nil || !strings.HasPrefix(data, "MY_TCP_PORT:") {
			continue
		}
		advertised := strings.TrimPrefix(data, "MY_TCP_PORT:")
		_, port, err := net.SplitHostPort(advertised)
		if err != nil {
			continue
		}
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			continue
		}
		devices = append(devices, Device{IP: ip, Address: net.JoinHostPort(ip, port)})
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Address < devices[j].Address })
	u.mu.Lock()
	u.devices = devices
	u.mu.Unlock()
	u.emit(searchCtx, Event{Type: EventDevicesChanged, Devices: slices.Clone(devices)})
	return slices.Clone(devices), nil
}

func (u *Usecase) ListDevices() []Device {
	u.mu.Lock()
	defer u.mu.Unlock()
	return slices.Clone(u.devices)
}

func (u *Usecase) newSession(r *run, conn *connect.Connection, incoming bool) *session {
	u.nextID++
	ctx, cancel := context.WithCancel(r.ctx)
	status := StatusConnecting
	if incoming {
		status = StatusAwaitingApproval
	}
	s := &session{ctx: ctx, finished: make(chan struct{}), info: ConnectionInfo{ID: strconv.FormatUint(u.nextID, 10), Address: conn.RemoteAddr, Incoming: incoming, Status: status}, conn: conn, cancel: cancel, ready: make(chan struct{})}
	u.sessions[s.info.ID] = s
	r.wg.Add(1)
	go func() { defer r.wg.Done(); u.monitor(ctx, s) }()
	return s
}

func (u *Usecase) incoming(r *run, conn *connect.Connection) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if r.ctx.Err() != nil {
		conn.Close()
		return
	}
	u.newSession(r, conn, true)
}

func (u *Usecase) Connect(ctx context.Context, addr string) (ConnectionInfo, error) {
	host, port, err := net.SplitHostPort(addr)
	number, portErr := strconv.Atoi(port)
	if err != nil || host == "" || portErr != nil || number < 1 || number > 65535 {
		return ConnectionInfo{}, fmt.Errorf("invalid address: %q", addr)
	}
	if err := ctx.Err(); err != nil {
		return ConnectionInfo{}, err
	}
	u.mu.Lock()
	r := u.current
	if r == nil || r.ctx.Err() != nil {
		u.mu.Unlock()
		return ConnectionInfo{}, ErrNotRunning
	}
	for _, s := range u.sessions {
		if s.info.Address == addr && s.info.Status != StatusDisconnected {
			u.mu.Unlock()
			return ConnectionInfo{}, ErrAlreadyConnected
		}
	}
	conn := connect.NewConnection(addr)
	s := u.newSession(r, conn, false)
	r.wg.Add(1)
	go func() { defer r.wg.Done(); u.con.ClientConnect(s.ctx, addr, conn) }()
	u.mu.Unlock()
	select {
	case <-ctx.Done():
		s.cancel()
		conn.Close()
		<-s.finished
		return ConnectionInfo{}, ctx.Err()
	case <-r.ctx.Done():
		return ConnectionInfo{}, r.ctx.Err()
	case <-s.ready:
		u.mu.Lock()
		info := s.info
		u.mu.Unlock()
		return info, s.readyErr
	}
}

func (u *Usecase) monitor(ctx context.Context, s *session) {
	defer close(s.finished)
	u.mu.Lock()
	info := s.info
	u.mu.Unlock()
	kind := EventConnectionChanged
	if info.Incoming {
		kind = EventConnectionRequested
	}
	u.emit(ctx, Event{Type: kind, Connection: info})
	var err error
	select {
	case err = <-s.conn.Ready:
	case <-ctx.Done():
		err = ctx.Err()
	}
	u.mu.Lock()
	s.readyErr = err
	if err == nil {
		s.info.Status = StatusConnected
	} else {
		s.info.Status = StatusDisconnected
	}
	info = s.info
	close(s.ready)
	u.mu.Unlock()
	if err == nil {
		u.emit(ctx, Event{Type: EventConnectionChanged, Connection: info})
	loop:
		for {
			select {
			case data := <-s.conn.Recv:
				u.emit(ctx, Event{Type: EventMessageReceived, Connection: info, Message: strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")})
			case <-s.conn.Done:
				break loop
			case <-ctx.Done():
				break loop
			}
		}
	}
	s.conn.Close()
	<-s.conn.Done
	if err == nil {
		err = s.conn.Err
	}
	u.mu.Lock()
	s.info.Status = StatusDisconnected
	info = s.info
	u.mu.Unlock()
	if err != nil && ctx.Err() == nil && !errors.Is(err, connect.ErrRejected) {
		u.emit(ctx, Event{Type: EventError, Connection: info, Error: err.Error()})
	}
	u.emit(ctx, Event{Type: EventConnectionChanged, Connection: info})
	s.cancel()
}

func (u *Usecase) ListConnections() []ConnectionInfo {
	u.mu.Lock()
	defer u.mu.Unlock()
	result := make([]ConnectionInfo, 0, len(u.sessions))
	for _, s := range u.sessions {
		result = append(result, s.info)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (u *Usecase) ListPendingRequests() []ConnectionRequest {
	u.mu.Lock()
	defer u.mu.Unlock()
	result := make([]ConnectionRequest, 0)
	for _, s := range u.sessions {
		if s.info.Incoming && s.info.Status == StatusAwaitingApproval && !s.decided {
			result = append(result, ConnectionRequest{ID: s.info.ID, Address: s.info.Address})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (u *Usecase) AcceptConnection(id string) error { return u.decide(id, true) }
func (u *Usecase) RejectConnection(id string) error { return u.decide(id, false) }
func (u *Usecase) decide(id string, accept bool) error {
	u.mu.Lock()
	r := u.current
	if r == nil || r.ctx.Err() != nil {
		u.mu.Unlock()
		return ErrNotRunning
	}
	s := u.sessions[id]
	if s == nil {
		u.mu.Unlock()
		return ErrNotFound
	}
	if !s.info.Incoming || s.info.Status != StatusAwaitingApproval || s.decided {
		u.mu.Unlock()
		return ErrInvalidState
	}
	s.decided = true
	u.mu.Unlock()
	decision := "REJECTED"
	if accept {
		decision = "ACCEPTED"
	}
	select {
	case s.conn.Send <- []byte(decision):
	case <-s.conn.Done:
		return ErrInvalidState
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
	if !accept {
		return nil
	}
	select {
	case <-s.ready:
		return s.readyErr
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
}

func (u *Usecase) Disconnect(id string) error {
	u.mu.Lock()
	s := u.sessions[id]
	if s == nil {
		u.mu.Unlock()
		return ErrNotFound
	}
	s.cancel()
	s.conn.Close()
	u.mu.Unlock()
	<-s.finished
	return nil
}

func (u *Usecase) SendMessage(ctx context.Context, id, text string) error {
	if strings.ContainsAny(text, "\r\n") {
		return fmt.Errorf("message must contain a single line")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	u.mu.Lock()
	r := u.current
	if r == nil || r.ctx.Err() != nil {
		u.mu.Unlock()
		return ErrNotRunning
	}
	s := u.sessions[id]
	if s == nil {
		u.mu.Unlock()
		return ErrNotFound
	}
	if s.info.Status != StatusConnected {
		u.mu.Unlock()
		return ErrInvalidState
	}
	u.mu.Unlock()
	select {
	case s.conn.Send <- []byte(text):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.conn.Done:
		return ErrInvalidState
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
}

// Running reports whether listeners and sessions are allowed to run.
func (u *Usecase) Running() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.current != nil && u.current.ctx.Err() == nil
}
